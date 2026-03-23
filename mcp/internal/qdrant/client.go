package qdrant

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	qdrant "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	collectionName    string
	conn              *grpc.ClientConn
	pointsClient      qdrant.PointsClient
	collectionsClient qdrant.CollectionsClient
}

type Memory struct {
	ID         uint64 // Original memory ID (same for all replicas)
	PointID    uint64 // Unique point ID in Qdrant (different for each embedding replica)
	Project    string
	Context    string
	Content    string
	Keywords   []string
	Scope      string
	Embeddings map[string][]float32 // Named vectors (e.g., "main", "project", "context", "content", "keyword1", ...)
}

type SearchResult struct {
	Memory     Memory
	Similarity float32
}

func NewClient(host, collectionName string) (*Client, error) {
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	return &Client{
		collectionName:    collectionName,
		conn:              conn,
		pointsClient:      qdrant.NewPointsClient(conn),
		collectionsClient: qdrant.NewCollectionsClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) DeleteCollection(ctx context.Context, collectionName string) error {
	_, err := c.collectionsClient.Delete(ctx, &qdrant.DeleteCollection{
		CollectionName: collectionName,
	})
	return err
}

func (c *Client) EnsureCollection(ctx context.Context, vectorSize uint64) error {
	collections, err := c.collectionsClient.List(ctx, &qdrant.ListCollectionsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	collectionExists := false
	for _, collection := range collections.GetCollections() {
		if collection.GetName() == c.collectionName {
			collectionExists = true
			break
		}
	}

	if collectionExists {
		// Collection already exists - assume it has the correct configuration
		// (If it doesn't, upsert will fail with vector name error)
		// We could try to update the collection here, but for simplicity
		// we'll assume it's either already multi-vector or will be recreated.
		log.Printf("Collection %q already exists", c.collectionName)
		return nil
	}

	// Define named vectors for multi-embedding memory indexing
	// We support: main, project, context, content, scope, keyword1, keyword2, keyword3, keyword4, keyword5
	// All have the same size and distance
	paramsMap := make(map[string]*qdrant.VectorParams)
	namedVectors := []string{VectorMain, VectorProject, VectorContext, VectorContent, VectorScope, VectorKeyword1, VectorKeyword2, VectorKeyword3, VectorKeyword4, VectorKeyword5}
	for _, name := range namedVectors {
		paramsMap[name] = &qdrant.VectorParams{
			Size:     vectorSize,
			Distance: qdrant.Distance_Cosine,
		}
	}

	_, err = c.collectionsClient.Create(ctx, &qdrant.CreateCollection{
		CollectionName: c.collectionName,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_ParamsMap{
				ParamsMap: &qdrant.VectorParamsMap{
					Map: paramsMap,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	log.Printf("Created collection %q with named vectors: %v", c.collectionName, namedVectors)
	return nil
}

func (c *Client) UpsertMemory(ctx context.Context, memory Memory) error {
	return c.UpsertMemories(ctx, []Memory{memory})
}

func (c *Client) UpsertMemories(ctx context.Context, memories []Memory) error {
	if len(memories) == 0 {
		return nil
	}

	// Create points for each memory (each memory is a single point with multiple named vectors)
	points := make([]*qdrant.PointStruct, 0, len(memories))
	for _, memory := range memories {
		// Build payload
		keywordValues := make([]*qdrant.Value, 0, len(memory.Keywords))
		for _, kw := range memory.Keywords {
			keywordValues = append(keywordValues, &qdrant.Value{
				Kind: &qdrant.Value_StringValue{
					StringValue: kw,
				},
			})
		}

		payload := map[string]*qdrant.Value{
			"project": {
				Kind: &qdrant.Value_StringValue{
					StringValue: memory.Project,
				},
			},
			"context": {
				Kind: &qdrant.Value_StringValue{
					StringValue: memory.Context,
				},
			},
			"content": {
				Kind: &qdrant.Value_StringValue{
					StringValue: memory.Content,
				},
			},
			"keywords": {
				Kind: &qdrant.Value_ListValue{
					ListValue: &qdrant.ListValue{
						Values: keywordValues,
					},
				},
			},
			"scope": {
				Kind: &qdrant.Value_StringValue{
					StringValue: memory.Scope,
				},
			},
			"memory_id": {
				Kind: &qdrant.Value_StringValue{
					StringValue: fmt.Sprintf("%d", memory.ID),
				},
			},
		}

		// Create named vectors from Embeddings map
		namedVectors := &qdrant.NamedVectors{
			Vectors: make(map[string]*qdrant.Vector),
		}
		for name, embedding := range memory.Embeddings {
			namedVectors.Vectors[name] = &qdrant.Vector{Data: embedding}
		}

		// Ensure we have at least one embedding
		if len(namedVectors.Vectors) == 0 {
			return fmt.Errorf("memory %d has no embeddings", memory.ID)
		}

		point := &qdrant.PointStruct{
			Id: &qdrant.PointId{
				PointIdOptions: &qdrant.PointId_Num{
					Num: memory.PointID,
				},
			},
			Vectors: &qdrant.Vectors{
				VectorsOptions: &qdrant.Vectors_Vectors{
					Vectors: namedVectors,
				},
			},
			Payload: payload,
		}
		points = append(points, point)
	}

	_, err := c.pointsClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: c.collectionName,
		Points:         points,
	})
	return err
}

func (c *Client) SearchMemories(ctx context.Context, embedding []float32, limit uint64, scoreThreshold float32, vectorName string) ([]SearchResult, error) {
	searchReq := &qdrant.SearchPoints{
		CollectionName: c.collectionName,
		Vector:         embedding,
		Limit:          limit,
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
		ScoreThreshold: &scoreThreshold,
	}
	if vectorName != "" {
		searchReq.VectorName = &vectorName
	}
	resp, err := c.pointsClient.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	var results []SearchResult
	for _, result := range resp.GetResult() {
		payload := result.GetPayload()

		project := ""
		context := ""
		content := ""
		scope := DefaultScope()
		var keywords []string
		memoryID := result.GetId().GetNum() // Default to point ID

		if projectVal, ok := payload["project"]; ok {
			project = projectVal.GetStringValue()
		}
		if contextVal, ok := payload["context"]; ok {
			context = contextVal.GetStringValue()
		}
		if contentVal, ok := payload["content"]; ok {
			content = contentVal.GetStringValue()
		}
		if keywordsVal, ok := payload["keywords"]; ok {
			if listValue := keywordsVal.GetListValue(); listValue != nil {
				for _, v := range listValue.GetValues() {
					keywords = append(keywords, v.GetStringValue())
				}
			}
		}
		if scopeVal, ok := payload["scope"]; ok {
			scope = scopeVal.GetStringValue()
		}
		if memoryIDVal, ok := payload["memory_id"]; ok {
			if idStr := memoryIDVal.GetStringValue(); idStr != "" {
				if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
					memoryID = id
				}
			}
		}

		results = append(results, SearchResult{
			Memory: Memory{
				ID:         memoryID,
				Project:    project,
				Context:    context,
				Content:    content,
				Keywords:   keywords,
				Scope:      scope,
				Embeddings: nil,
			},
			Similarity: result.GetScore(),
		})
	}

	return results, nil
}

func (c *Client) SearchMemoriesBatch(ctx context.Context, embeddings [][]float32, limit uint64, scoreThreshold float32, vectorNames []string) ([][]SearchResult, error) {
	if len(embeddings) != len(vectorNames) {
		return nil, fmt.Errorf("embeddings and vectorNames must have the same length")
	}
	if len(embeddings) == 0 {
		return nil, nil
	}

	// Build search points for each embedding
	searchPoints := make([]*qdrant.SearchPoints, 0, len(embeddings))
	for i, embedding := range embeddings {
		sp := &qdrant.SearchPoints{
			CollectionName: c.collectionName,
			Vector:         embedding,
			Limit:          limit,
			WithPayload: &qdrant.WithPayloadSelector{
				SelectorOptions: &qdrant.WithPayloadSelector_Enable{
					Enable: true,
				},
			},
			ScoreThreshold: &scoreThreshold,
		}
		if vectorNames[i] != "" {
			sp.VectorName = &vectorNames[i]
		}
		searchPoints = append(searchPoints, sp)
	}

	batchReq := &qdrant.SearchBatchPoints{
		CollectionName: c.collectionName,
		SearchPoints:   searchPoints,
	}
	resp, err := c.pointsClient.SearchBatch(ctx, batchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search batch: %w", err)
	}

	// Process each batch result
	allResults := make([][]SearchResult, len(resp.GetResult()))
	for batchIdx, batchResult := range resp.GetResult() {
		var results []SearchResult
		for _, scoredPoint := range batchResult.GetResult() {
			payload := scoredPoint.GetPayload()

			project := ""
			context := ""
			content := ""
			scope := DefaultScope()
			var keywords []string
			memoryID := scoredPoint.GetId().GetNum() // Default to point ID

			if projectVal, ok := payload["project"]; ok {
				project = projectVal.GetStringValue()
			}
			if contextVal, ok := payload["context"]; ok {
				context = contextVal.GetStringValue()
			}
			if contentVal, ok := payload["content"]; ok {
				content = contentVal.GetStringValue()
			}
			if keywordsVal, ok := payload["keywords"]; ok {
				if listValue := keywordsVal.GetListValue(); listValue != nil {
					for _, v := range listValue.GetValues() {
						keywords = append(keywords, v.GetStringValue())
					}
				}
			}
			if scopeVal, ok := payload["scope"]; ok {
				scope = scopeVal.GetStringValue()
			}
			if memoryIDVal, ok := payload["memory_id"]; ok {
				if idStr := memoryIDVal.GetStringValue(); idStr != "" {
					if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
						memoryID = id
					}
				}
			}

			results = append(results, SearchResult{
				Memory: Memory{
					ID:         memoryID,
					Project:    project,
					Context:    context,
					Content:    content,
					Keywords:   keywords,
					Scope:      scope,
					Embeddings: nil,
				},
				Similarity: scoredPoint.GetScore(),
			})
		}
		allResults[batchIdx] = results
	}

	return allResults, nil
}

func (c *Client) DeleteMemory(ctx context.Context, memoryID uint64) error {
	// First, find all points with this memory_id
	// We need to search for the memory_id in payload to get all point IDs
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: "memory_id",
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Keyword{
								Keyword: fmt.Sprintf("%d", memoryID),
							},
						},
					},
				},
			},
		},
	}

	// Get all points with this memory_id
	var offset uint64
	var allPointIDs []uint64
	limit := uint32(100)

	for {
		req := &qdrant.ScrollPoints{
			CollectionName: c.collectionName,
			Filter:         filter,
			Limit:          &limit,
			WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: false}},
			WithVectors:    &qdrant.WithVectorsSelector{SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: false}},
		}
		if offset > 0 {
			req.Offset = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: offset}}
		}

		resp, err := c.pointsClient.Scroll(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to scroll memories for deletion: %w", err)
		}

		points := resp.GetResult()
		if len(points) == 0 {
			break
		}

		for _, point := range points {
			allPointIDs = append(allPointIDs, point.GetId().GetNum())
			offset = point.GetId().GetNum() + 1
		}

		if len(points) < int(limit) {
			break
		}
	}

	// Delete all points with this memory_id
	if len(allPointIDs) > 0 {
		pointIDs := make([]*qdrant.PointId, len(allPointIDs))
		for i, id := range allPointIDs {
			pointIDs[i] = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: id}}
		}

		_, err := c.pointsClient.Delete(ctx, &qdrant.DeletePoints{
			CollectionName: c.collectionName,
			Points:         qdrant.NewPointsSelector(pointIDs...),
		})
		if err != nil {
			return fmt.Errorf("failed to delete memory points: %w", err)
		}
	} else {
		log.Printf("Warning: No points found for memory_id %d", memoryID)
	}

	return nil
}

func (c *Client) DeleteMemoriesByProject(ctx context.Context, project string) error {
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: "project",
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Keyword{
								Keyword: project,
							},
						},
					},
				},
			},
		},
	}

	// Delete in batches using scroll and delete
	var offset uint64
	limit := uint32(100)

	for {
		req := &qdrant.ScrollPoints{
			CollectionName: c.collectionName,
			Filter:         filter,
			Limit:          &limit,
			WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: false}},
			WithVectors:    &qdrant.WithVectorsSelector{SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: false}},
		}
		if offset > 0 {
			req.Offset = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: offset}}
		}

		resp, err := c.pointsClient.Scroll(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to scroll memories for deletion: %w", err)
		}

		points := resp.GetResult()
		if len(points) == 0 {
			break
		}

		// Collect point IDs for this batch
		pointIDs := make([]*qdrant.PointId, len(points))
		for i, point := range points {
			pointIDs[i] = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: point.GetId().GetNum()}}
			offset = point.GetId().GetNum() + 1
		}

		// Delete this batch
		_, err = c.pointsClient.Delete(ctx, &qdrant.DeletePoints{
			CollectionName: c.collectionName,
			Points:         qdrant.NewPointsSelector(pointIDs...),
		})
		if err != nil {
			return fmt.Errorf("failed to delete memory batch: %w", err)
		}

		if len(points) < int(limit) {
			break
		}
	}

	return nil
}

func (c *Client) DeleteAllMemories(ctx context.Context) error {
	// Delete all points by scrolling through all points without any filter
	// This preserves the collection and its vector dimension
	var offset uint64
	limit := uint32(100)

	for {
		req := &qdrant.ScrollPoints{
			CollectionName: c.collectionName,
			Limit:          &limit,
			WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: false}},
			WithVectors:    &qdrant.WithVectorsSelector{SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: false}},
		}
		if offset > 0 {
			req.Offset = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: offset}}
		}

		resp, err := c.pointsClient.Scroll(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to scroll memories for deletion: %w", err)
		}

		points := resp.GetResult()
		if len(points) == 0 {
			break
		}

		// Collect point IDs for this batch
		pointIDs := make([]*qdrant.PointId, len(points))
		for i, point := range points {
			pointIDs[i] = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: point.GetId().GetNum()}}
			offset = point.GetId().GetNum() + 1
		}

		// Delete this batch
		_, err = c.pointsClient.Delete(ctx, &qdrant.DeletePoints{
			CollectionName: c.collectionName,
			Points:         qdrant.NewPointsSelector(pointIDs...),
		})
		if err != nil {
			return fmt.Errorf("failed to delete memory batch: %w", err)
		}

		if len(points) < int(limit) {
			break
		}
	}

	return nil
}

func (c *Client) ListMemories(ctx context.Context, filterProject string, filterKeywords []string, limit uint64) ([]Memory, error) {
	// Build filter for project only (keywords filtered client-side)
	var filter *qdrant.Filter
	if filterProject != "" {
		filter = &qdrant.Filter{
			Must: []*qdrant.Condition{
				{
					ConditionOneOf: &qdrant.Condition_Field{
						Field: &qdrant.FieldCondition{
							Key: "project",
							Match: &qdrant.Match{
								MatchValue: &qdrant.Match_Keyword{
									Keyword: filterProject,
								},
							},
						},
					},
				},
			},
		}
	}

	// Use map to deduplicate by memory_id (since we have multiple points per memory)
	memoryMap := make(map[uint64]Memory)
	var offset *qdrant.PointId
	batchSize := uint32(100)

	for uint64(len(memoryMap)) < limit {
		req := &qdrant.ScrollPoints{
			CollectionName: c.collectionName,
			Limit:          &batchSize,
			WithPayload: &qdrant.WithPayloadSelector{
				SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true},
			},
			WithVectors: &qdrant.WithVectorsSelector{
				SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: false},
			},
		}
		if filter != nil {
			req.Filter = filter
		}
		if offset != nil {
			req.Offset = offset
		}

		resp, err := c.pointsClient.Scroll(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to scroll memories: %w", err)
		}

		points := resp.GetResult()
		if len(points) == 0 {
			break
		}

		for _, point := range points {
			payload := point.GetPayload()

			project := ""
			context := ""
			content := ""
			scope := DefaultScope()
			var keywords []string
			memoryID := point.GetId().GetNum()

			if projectVal, ok := payload["project"]; ok {
				project = projectVal.GetStringValue()
			}
			if contextVal, ok := payload["context"]; ok {
				context = contextVal.GetStringValue()
			}
			if contentVal, ok := payload["content"]; ok {
				content = contentVal.GetStringValue()
			}
			if keywordsVal, ok := payload["keywords"]; ok {
				if listValue := keywordsVal.GetListValue(); listValue != nil {
					for _, v := range listValue.GetValues() {
						keywords = append(keywords, v.GetStringValue())
					}
				}
			}
			if scopeVal, ok := payload["scope"]; ok {
				scope = scopeVal.GetStringValue()
			}
			if memoryIDVal, ok := payload["memory_id"]; ok {
				if idStr := memoryIDVal.GetStringValue(); idStr != "" {
					if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
						memoryID = id
					}
				}
			}

			if _, exists := memoryMap[memoryID]; !exists {
				memoryMap[memoryID] = Memory{
					ID:       memoryID,
					Project:  project,
					Context:  context,
					Content:  content,
					Keywords: keywords,
					Scope:    scope,
				}
			}
		}

		lastPoint := points[len(points)-1]
		lastID := lastPoint.GetId().GetNum()
		offset = &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: lastID + 1}}

		if len(points) < int(batchSize) {
			break
		}
	}

	// Convert map to slice
	memories := make([]Memory, 0, len(memoryMap))
	for _, memory := range memoryMap {
		memories = append(memories, memory)
	}

	// Apply limit
	if uint64(len(memories)) > limit {
		memories = memories[:limit]
	}

	// Filter by keywords client-side if provided
	if len(filterKeywords) > 0 {
		var filtered []Memory
		for _, memory := range memories {
			memoryKeywordsLower := make([]string, len(memory.Keywords))
			for i, kw := range memory.Keywords {
				memoryKeywordsLower[i] = strings.ToLower(kw)
			}
			matches := false
			for _, filterKw := range filterKeywords {
				filterKwLower := strings.ToLower(filterKw)
				for _, memoryKw := range memoryKeywordsLower {
					if memoryKw == filterKwLower {
						matches = true
						break
					}
				}
				if matches {
					break
				}
			}
			if matches {
				filtered = append(filtered, memory)
			}
		}
		return filtered, nil
	}

	return memories, nil
}
