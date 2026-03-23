package mcp

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/augustohdias/hippocampus/mcp/internal/config"
	"github.com/augustohdias/hippocampus/mcp/internal/ollama"
	"github.com/augustohdias/hippocampus/mcp/internal/qdrant"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Service struct {
	cfg    *config.Config
	ollama *ollama.Client
	qdrant *qdrant.Client
}

func NewService(cfg *config.Config) (*Service, error) {
	ollamaClient := ollama.NewClient(cfg.OllamaHost, cfg.OllamaModel)
	qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
	}

	return &Service{
		cfg:    cfg,
		ollama: ollamaClient,
		qdrant: qdrantClient,
	}, nil
}

func (s *Service) Close() error {
	if s.qdrant != nil {
		return s.qdrant.Close()
	}
	return nil
}

func (s *Service) EnsureCollection(ctx context.Context) error {
	embedding, err := s.ollama.GetEmbedding("test")
	if err != nil {
		return fmt.Errorf("failed to get test embedding for vector size: %w", err)
	}

	vectorSize := uint64(len(embedding))
	return s.qdrant.EnsureCollection(ctx, vectorSize)
}

func (s *Service) CreateMemory(ctx context.Context, project, context, content string, keywords []string, scope string) (uint64, error) {
	memoryID := generateID()

	// Validate and set default scope
	if scope == "" {
		scope = qdrant.DefaultScope()
	}
	if !qdrant.ValidScope(scope) {
		return 0, fmt.Errorf("invalid scope: %s, must be one of: %s, %s, %s", scope, qdrant.ScopeGlobal, qdrant.ScopePersonal, qdrant.ScopeProject)
	}

	// Build list of texts to generate embeddings for
	var embeddingTexts []string
	var vectorNames []string

	// Determine if we should include project in main embedding
	includeProjectInMain := scope == qdrant.ScopeProject

	// Main embedding (combined)
	var mainText string
	if includeProjectInMain {
		// For project scope: include project in main embedding
		mainText = fmt.Sprintf("<%s>\n<%s>\n<%s>\n<%s>",
			project,
			context,
			content,
			strings.Join(keywords, ", "))
	} else {
		// For global/personal scope: exclude project from main embedding
		mainText = fmt.Sprintf("<%s>\n<%s>\n<%s>",
			context,
			content,
			strings.Join(keywords, ", "))
	}
	embeddingTexts = append(embeddingTexts, mainText)
	vectorNames = append(vectorNames, qdrant.VectorMain)

	// Project parameter embedding (always)
	if project != "" {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", project))
		vectorNames = append(vectorNames, qdrant.VectorProject)
	}

	// Context parameter embedding (if provided)
	if context != "" {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", context))
		vectorNames = append(vectorNames, qdrant.VectorContext)
	}

	// Content parameter embedding (always required, but check anyway)
	if content != "" {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", content))
		vectorNames = append(vectorNames, qdrant.VectorContent)
	}

	// Scope embedding (for global and personal scopes)
	if scope == qdrant.ScopeGlobal || scope == qdrant.ScopePersonal {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", scope))
		vectorNames = append(vectorNames, qdrant.VectorScope)
	}

	// First 5 keywords (each individually, 1-indexed)
	maxKeywords := 5
	if len(keywords) < maxKeywords {
		maxKeywords = len(keywords)
	}
	for i := 0; i < maxKeywords; i++ {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", keywords[i]))
		// Use keyword1, keyword2, ... keyword5
		vectorNames = append(vectorNames, fmt.Sprintf("keyword%d", i+1))
	}

	// Generate all embeddings
	embeddings := make([][]float32, 0, len(embeddingTexts))
	for _, text := range embeddingTexts {
		embedding, err := s.ollama.GetEmbedding(text)
		if err != nil {
			return 0, fmt.Errorf("failed to get embedding for %q: %w", text, err)
		}
		embeddings = append(embeddings, embedding)
	}

	// Create a single memory with multiple named vectors
	embeddingsMap := make(map[string][]float32)
	for i, embedding := range embeddings {
		embeddingsMap[vectorNames[i]] = embedding
	}
	memory := qdrant.Memory{
		ID:         memoryID,
		PointID:    memoryID, // Single point ID (same as memory ID)
		Project:    project,
		Context:    context,
		Content:    content,
		Keywords:   keywords,
		Scope:      scope,
		Embeddings: embeddingsMap,
	}

	if err := s.qdrant.UpsertMemories(ctx, []qdrant.Memory{memory}); err != nil {
		return 0, fmt.Errorf("failed to upsert memories: %w", err)
	}

	return memoryID, nil
}

func (s *Service) SearchMemories(ctx context.Context, project, context, keywords, scope string, limit uint64) ([]qdrant.SearchResult, error) {
	// Build list of query embeddings for better matching with corresponding vector names:
	// 1. Combined query -> "main"
	// 2. Project parameter -> "project"
	// 3. Context parameter -> "context" (if provided)
	// 4. Scope-only -> "scope" (if scope is global or personal)
	// 5. Each keyword individually -> "keyword1", "keyword2", ... up to 5
	type query struct {
		text       string
		vectorName string
	}
	var queries []query

	// Combined query (main)
	queryParts := []string{project}
	if context != "" {
		queryParts = append(queryParts, context)
	}
	if keywords != "" {
		queryParts = append(queryParts, keywords)
	}
	// Include scope in query if provided (helps match scope-specific embeddings)
	if scope != "" && (scope == qdrant.ScopeGlobal || scope == qdrant.ScopePersonal) {
		queryParts = append(queryParts, scope)
	}
	var wrappedParts []string
	for _, part := range queryParts {
		wrappedParts = append(wrappedParts, "<"+part+">")
	}
	queries = append(queries, query{
		text:       strings.Join(wrappedParts, "\n"),
		vectorName: qdrant.VectorMain,
	})

	// Project parameter query (always if project provided)
	if project != "" {
		queries = append(queries, query{
			text:       fmt.Sprintf("<%s>", project),
			vectorName: qdrant.VectorProject,
		})
	}

	// Context parameter query (if provided)
	if context != "" {
		queries = append(queries, query{
			text:       fmt.Sprintf("<%s>", context),
			vectorName: qdrant.VectorContext,
		})
	}

	// Scope-only query (for global/personal scopes)
	if scope == qdrant.ScopeGlobal || scope == qdrant.ScopePersonal {
		queries = append(queries, query{
			text:       fmt.Sprintf("<%s>", scope),
			vectorName: qdrant.VectorScope,
		})
	}

	// Individual keyword queries (up to 5 keywords)
	if keywords != "" {
		rawKeywords := strings.Split(keywords, ",")
		maxKeywords := 5
		if len(rawKeywords) < maxKeywords {
			maxKeywords = len(rawKeywords)
		}
		for i := 0; i < maxKeywords; i++ {
			trimmed := strings.TrimSpace(rawKeywords[i])
			if trimmed != "" {
				queries = append(queries, query{
					text:       fmt.Sprintf("<%s>", trimmed),
					vectorName: fmt.Sprintf("keyword%d", i+1),
				})
			}
		}
	}

	// Generate all query embeddings
	type queryEmbedding struct {
		embedding  []float32
		vectorName string
	}
	var queryEmbeddings []queryEmbedding
	for _, q := range queries {
		embedding, err := s.ollama.GetEmbedding(q.text)
		if err != nil {
			return nil, fmt.Errorf("failed to get embedding for %q: %w", q.text, err)
		}
		queryEmbeddings = append(queryEmbeddings, queryEmbedding{
			embedding:  embedding,
			vectorName: q.vectorName,
		})
	}

	// Search with each embedding (using corresponding vector name) and merge results
	scoreThreshold := s.cfg.SearchScoreThreshold
	allResults := make(map[uint64]qdrant.SearchResult) // Deduplicate by memory ID, keep highest score

	// Prepare slices for batch search
	embeddings := make([][]float32, 0, len(queryEmbeddings))
	vectorNames := make([]string, 0, len(queryEmbeddings))
	for _, qe := range queryEmbeddings {
		embeddings = append(embeddings, qe.embedding)
		vectorNames = append(vectorNames, qe.vectorName)
	}

	// Perform batch search
	resultsBatch, err := s.qdrant.SearchMemoriesBatch(ctx, embeddings, limit, scoreThreshold, vectorNames)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}

	// Merge results from each batch (each batch corresponds to a query embedding)
	for _, batchResults := range resultsBatch {
		for _, result := range batchResults {
			existing, exists := allResults[result.Memory.ID]
			if !exists || result.Similarity > existing.Similarity {
				allResults[result.Memory.ID] = result
			}
		}
	}

	// Convert map to slice and sort by similarity (descending)
	results := make([]qdrant.SearchResult, 0, len(allResults))
	for _, result := range allResults {
		results = append(results, result)
	}

	// Sort by similarity descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Filter by scope if provided
	if scope != "" {
		filtered := make([]qdrant.SearchResult, 0, len(results))
		for _, result := range results {
			if result.Memory.Scope == scope {
				filtered = append(filtered, result)
			}
		}
		results = filtered
	}

	// Apply limit
	if uint64(len(results)) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *Service) ListMemories(ctx context.Context, filterProject string, filterKeywords []string, scope string, limit uint64) ([]qdrant.Memory, error) {
	memories, err := s.qdrant.ListMemories(ctx, filterProject, filterKeywords, limit)
	if err != nil {
		return nil, err
	}
	// Filter by scope if provided
	if scope != "" {
		filtered := make([]qdrant.Memory, 0, len(memories))
		for _, memory := range memories {
			if memory.Scope == scope {
				filtered = append(filtered, memory)
			}
		}
		memories = filtered
	}
	return memories, nil
}

func (s *Service) DeleteMemory(ctx context.Context, memoryID uint64) error {
	return s.qdrant.DeleteMemory(ctx, memoryID)
}

func (s *Service) DeleteMemoriesByProject(ctx context.Context, project string) error {
	return s.qdrant.DeleteMemoriesByProject(ctx, project)
}

func (s *Service) DeleteAllMemories(ctx context.Context) error {
	return s.qdrant.DeleteAllMemories(ctx)
}

func generateID() uint64 {
	return rand.Uint64()
}

func (s *Service) SetupTools(server *mcp.Server) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_memory",
		Description: "Creates a new memory with structured fields (project, context, content, keywords), generates embeddings and stores in vector database",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project": map[string]any{
					"type":        "string",
					"description": "Project identifier (current working directory when memory was written)",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "Brief context description (e.g., 'Code Patterns', 'Research Sources', 'Personal Information')",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The actual information to remember (max 250 words)",
				},
				"keywords": map[string]any{
					"type":        "string",
					"description": "Comma-separated keywords to help find the memory (max 20 keywords)",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "Scope of the memory: 'project' (default if omitted), 'personal', or 'global'",
				},
			},
			"required": []string{"content"},
		},
	}, s.handleCreateMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_memories",
		Description: "Search for memories by semantic similarity. All parameters are optional, but if scope='project' then project is required. Returns closest matches with similarity percentage.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project": map[string]any{
					"type":        "string",
					"description": "Optional project identifier. Required when scope='project'.",
				},
				"context": map[string]any{
					"type":        "string",
					"description": "Optional context to filter search",
				},
				"keywords": map[string]any{
					"type":        "string",
					"description": "Optional comma-separated keywords to filter search",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "Optional scope to filter search: 'project', 'personal', or 'global'. No default - if omitted, search across all scopes.",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of results (default: 10)",
				},
			},
			"required": []string{},
		},
	}, s.handleSearchMemories)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_memories",
		Description: "List memories filtered by scope. Scope is required. If scope='project', project parameter is required. Keywords not allowed.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope": map[string]any{
					"type":        "string",
					"description": "Scope filter: 'project', 'personal', or 'global' (required)",
				},
				"project": map[string]any{
					"type":        "string",
					"description": "Project filter. Required when scope='project', not allowed for other scopes.",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of results (default: 50, max: 100)",
				},
			},
			"required": []string{"scope"},
		},
	}, s.handleListMemories)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_memory",
		Description: "Delete a specific memory by its ID",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"memory_id": map[string]any{
					"type":        "string",
					"description": "The ID of the memory to delete",
				},
			},
			"required": []string{"memory_id"},
		},
	}, s.handleDeleteMemory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_memories_by_project",
		Description: "Delete all memories from a specific project",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project": map[string]any{
					"type":        "string",
					"description": "The project identifier to delete memories from",
				},
			},
			"required": []string{"project"},
		},
	}, s.handleDeleteMemoriesByProject)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_all_memories",
		Description: "Delete all memories stored (wipe entire collection)",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, s.handleDeleteAllMemories)

	return nil
}

func (s *Service) handleCreateMemory(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	project, _ := input["project"].(string)
	contextStr, _ := input["context"].(string)
	content, _ := input["content"].(string)
	keywordsStr, _ := input["keywords"].(string)
	scopeStr, _ := input["scope"].(string)

	// Validate required fields
	if content == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'content' is required"},
			},
		}, nil, nil
	}

	// Validate content length (max 250 words)
	wordCount := len(strings.Fields(content))
	if wordCount > 250 {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Content must be at most 250 words"},
			},
		}, nil, nil
	}

	// Parse keywords (comma-separated, max 20)
	var keywords []string
	if keywordsStr != "" {
		rawKeywords := strings.Split(keywordsStr, ",")
		for _, kw := range rawKeywords {
			trimmed := strings.TrimSpace(kw)
			if trimmed != "" {
				keywords = append(keywords, trimmed)
			}
		}
		if len(keywords) > 20 {
			keywords = keywords[:20]
		}
	}

	// Determine scope
	scope := scopeStr
	if scope == "" {
		scope = qdrant.DefaultScope()
	}

	// Validate scope
	if !qdrant.ValidScope(scope) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)},
			},
		}, nil, nil
	}

	// Project is required only for project scope; ignored for global/personal
	if scope == qdrant.ScopeProject && project == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'project' is required when scope is 'project'"},
			},
		}, nil, nil
	}
	if scope != qdrant.ScopeProject {
		project = ""
	}

	id, err := s.CreateMemory(ctx, project, contextStr, content, keywords, scope)
	if err != nil {
		log.Printf("Error creating memory: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error creating memory: %v", err)},
			},
		}, nil, nil
	}

	return nil, map[string]any{
		"id":      fmt.Sprintf("%d", id),
		"message": "Memory created successfully",
	}, nil
}

func (s *Service) handleSearchMemories(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	project, _ := input["project"].(string)
	contextStr, _ := input["context"].(string)
	keywordsStr, _ := input["keywords"].(string)
	scopeStr, _ := input["scope"].(string)

	// Project is required only when scope is "project" (will be validated later)
	// For other scopes or no scope, project is optional

	limit := uint64(10)
	if limitVal, ok := input["limit"].(float64); ok {
		limit = uint64(limitVal)
	}

	// Determine scope (optional parameter)
	scope := scopeStr

	// Validate scope if provided
	if scope != "" && !qdrant.ValidScope(scope) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)},
			},
		}, nil, nil
	}

	// If scope is "project", project parameter is required
	if scope == qdrant.ScopeProject && project == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'project' is required when scope is 'project'"},
			},
		}, nil, nil
	}

	results, err := s.SearchMemories(ctx, project, contextStr, keywordsStr, scope, limit)
	if err != nil {
		log.Printf("Error searching memories: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error searching memories: %v", err)},
			},
		}, nil, nil
	}

	var formattedResults []map[string]any
	for _, result := range results {
		similarityPercent := result.Similarity * 100
		formattedResults = append(formattedResults, map[string]any{
			"id":         fmt.Sprintf("%d", result.Memory.ID),
			"project":    result.Memory.Project,
			"context":    result.Memory.Context,
			"content":    result.Memory.Content,
			"keywords":   result.Memory.Keywords,
			"scope":      result.Memory.Scope,
			"similarity": fmt.Sprintf("%.2f%%", similarityPercent),
			"score":      result.Similarity,
		})
	}

	return nil, map[string]any{
		"results": formattedResults,
		"total":   len(formattedResults),
	}, nil
}

func (s *Service) handleListMemories(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	project, _ := input["project"].(string)
	keywordsStr, _ := input["keywords"].(string)
	scopeStr, _ := input["scope"].(string)

	// Scope is required for list operation
	if scopeStr == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'scope' is required for list operation"},
			},
		}, nil, nil
	}

	scope := scopeStr

	// Validate scope
	if !qdrant.ValidScope(scope) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)},
			},
		}, nil, nil
	}

	// If scope is "project", project parameter is required
	if scope == qdrant.ScopeProject && project == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'project' is required when scope is 'project'"},
			},
		}, nil, nil
	}

	// If scope is not "project", project parameter should not be provided
	if scope != qdrant.ScopeProject && project != "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'project' is only allowed when scope is 'project'"},
			},
		}, nil, nil
	}

	// Keywords parameter is not allowed for list operation
	if keywordsStr != "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'keywords' is not allowed for list operation"},
			},
		}, nil, nil
	}

	limit := uint64(50)
	if limitVal, ok := input["limit"].(float64); ok {
		limit = uint64(limitVal)
		if limit > 100 {
			limit = 100
		}
	}

	memories, err := s.ListMemories(ctx, project, nil, scope, limit)
	if err != nil {
		log.Printf("Error listing memories: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error listing memories: %v", err)},
			},
		}, nil, nil
	}

	var formattedMemories []map[string]any
	for _, memory := range memories {
		formattedMemories = append(formattedMemories, map[string]any{
			"id":       fmt.Sprintf("%d", memory.ID),
			"project":  memory.Project,
			"context":  memory.Context,
			"content":  memory.Content,
			"keywords": memory.Keywords,
			"scope":    memory.Scope,
		})
	}

	return nil, map[string]any{
		"memories": formattedMemories,
		"total":    len(formattedMemories),
	}, nil
}

func (s *Service) handleDeleteMemory(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	memoryIDStr, ok := input["memory_id"].(string)
	if !ok {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'memory_id' is required and must be a string"},
			},
		}, nil, nil
	}

	memoryID, err := strconv.ParseUint(memoryIDStr, 10, 64)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid memory_id format: %v", err)},
			},
		}, nil, nil
	}

	if err := s.DeleteMemory(ctx, memoryID); err != nil {
		log.Printf("Error deleting memory: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error deleting memory: %v", err)},
			},
		}, nil, nil
	}

	return nil, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Memory %d deleted successfully", memoryID),
	}, nil
}

func (s *Service) handleDeleteMemoriesByProject(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	project, _ := input["project"].(string)

	if project == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'project' is required"},
			},
		}, nil, nil
	}

	if err := s.DeleteMemoriesByProject(ctx, project); err != nil {
		log.Printf("Error deleting memories by project: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error deleting memories by project: %v", err)},
			},
		}, nil, nil
	}

	return nil, map[string]any{
		"success": true,
		"message": fmt.Sprintf("All memories from project '%s' deleted successfully", project),
	}, nil
}

func (s *Service) handleDeleteAllMemories(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	if err := s.DeleteAllMemories(ctx); err != nil {
		log.Printf("Error deleting all memories: %v", err)
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error deleting all memories: %v", err)},
			},
		}, nil, nil
	}

	return nil, map[string]any{
		"success": true,
		"message": "All memories deleted successfully",
	}, nil
}
