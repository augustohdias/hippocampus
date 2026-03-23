// Package mcp provides the implementation of all tools
// that are available for HTTP and MCP servers.
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

// NewService initializes Ollama and Qdrant clients and returns a ready-to-use Service.
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

// Close releases the Qdrant client connection.
func (s *Service) Close() error {
	if s.qdrant != nil {
		return s.qdrant.Close()
	}
	return nil
}

// EnsureCollection probes Ollama for the embedding vector size and creates the Qdrant
// collection with the correct dimensions if it does not already exist.
func (s *Service) EnsureCollection(ctx context.Context) error {
	embedding, err := s.ollama.GetEmbedding("test")
	if err != nil {
		return fmt.Errorf("failed to get test embedding for vector size: %w", err)
	}

	vectorSize := uint64(len(embedding))
	return s.qdrant.EnsureCollection(ctx, vectorSize)
}

// CreateMemory validates the scope, builds multiple named embeddings (main, project, context,
// content, scope, and up to 5 individual keywords), and upserts a single Qdrant point
// with all vectors attached. Returns the generated memory ID on success.
func (s *Service) CreateMemory(ctx context.Context, project, context, content string, keywords []string, scope string) (uint64, error) {
	project = sanitizeProject(project)
	memoryID := generateID()

	if scope == "" {
		scope = qdrant.DefaultScope()
	}
	if !qdrant.ValidScope(scope) {
		return 0, fmt.Errorf("invalid scope: %s, must be one of: %s, %s, %s", scope, qdrant.ScopeGlobal, qdrant.ScopePersonal, qdrant.ScopeProject)
	}

	var embeddingTexts []string
	var vectorNames []string

	includeProjectInMain := scope == qdrant.ScopeProject

	var mainText string
	if includeProjectInMain {
		mainText = fmt.Sprintf("<%s>\n<%s>\n<%s>\n<%s>",
			project,
			context,
			content,
			strings.Join(keywords, ", "))
	} else {
		mainText = fmt.Sprintf("<%s>\n<%s>\n<%s>",
			context,
			content,
			strings.Join(keywords, ", "))
	}
	embeddingTexts = append(embeddingTexts, mainText)
	vectorNames = append(vectorNames, qdrant.VectorMain)

	if project != "" {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", project))
		vectorNames = append(vectorNames, qdrant.VectorProject)
	}

	if context != "" {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", context))
		vectorNames = append(vectorNames, qdrant.VectorContext)
	}

	if content != "" {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", content))
		vectorNames = append(vectorNames, qdrant.VectorContent)
	}

	if scope == qdrant.ScopeGlobal || scope == qdrant.ScopePersonal {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", scope))
		vectorNames = append(vectorNames, qdrant.VectorScope)
	}

	maxKeywords := 5
	maxKeywords = min(len(keywords), maxKeywords)

	for i := 0; i < maxKeywords; i++ {
		embeddingTexts = append(embeddingTexts, fmt.Sprintf("<%s>", keywords[i]))
		vectorNames = append(vectorNames, fmt.Sprintf("keyword%d", i+1))
	}

	embeddings := make([][]float32, 0, len(embeddingTexts))
	for _, text := range embeddingTexts {
		embedding, err := s.ollama.GetEmbedding(text)
		if err != nil {
			return 0, fmt.Errorf("failed to get embedding for %q: %w", text, err)
		}
		embeddings = append(embeddings, embedding)
	}

	embeddingsMap := make(map[string][]float32)
	for i, embedding := range embeddings {
		embeddingsMap[vectorNames[i]] = embedding
	}
	memory := qdrant.Memory{
		ID:         memoryID,
		PointID:    memoryID,
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

// SearchMemories builds a list of query embeddings for better matching with corresponding
// vector names: (1) combined query -> "main", (2) project -> "project",
// (3) context -> "context" if provided, (4) scope-only -> "scope" for global/personal,
// (5) each keyword individually -> "keyword1" ... "keyword5".
// Results are deduplicated by memory ID, keeping the highest similarity score.
func (s *Service) SearchMemories(ctx context.Context, project, context, keywords, scope string, limit uint64) ([]qdrant.SearchResult, error) {
	project = sanitizeProject(project)
	type query struct {
		text       string
		vectorName string
	}
	var queries []query

	queryParts := []string{project}
	if context != "" {
		queryParts = append(queryParts, context)
	}
	if keywords != "" {
		queryParts = append(queryParts, keywords)
	}
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

	if project != "" {
		queries = append(queries, query{
			text:       fmt.Sprintf("<%s>", project),
			vectorName: qdrant.VectorProject,
		})
	}

	if context != "" {
		queries = append(queries, query{
			text:       fmt.Sprintf("<%s>", context),
			vectorName: qdrant.VectorContext,
		})
	}

	if scope == qdrant.ScopeGlobal || scope == qdrant.ScopePersonal {
		queries = append(queries, query{
			text:       fmt.Sprintf("<%s>", scope),
			vectorName: qdrant.VectorScope,
		})
	}

	if keywords != "" {
		rawKeywords := strings.Split(keywords, ",")
		maxKeywords := 5
		maxKeywords = min(len(rawKeywords), maxKeywords)
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

	scoreThreshold := s.cfg.SearchScoreThreshold
	allResults := make(map[uint64]qdrant.SearchResult)

	embeddings := make([][]float32, 0, len(queryEmbeddings))
	vectorNames := make([]string, 0, len(queryEmbeddings))
	for _, qe := range queryEmbeddings {
		embeddings = append(embeddings, qe.embedding)
		vectorNames = append(vectorNames, qe.vectorName)
	}

	resultsBatch, err := s.qdrant.SearchMemoriesBatch(ctx, embeddings, limit, scoreThreshold, vectorNames)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}

	for _, batchResults := range resultsBatch {
		for _, result := range batchResults {
			existing, exists := allResults[result.Memory.ID]
			if !exists || result.Similarity > existing.Similarity {
				allResults[result.Memory.ID] = result
			}
		}
	}

	results := make([]qdrant.SearchResult, 0, len(allResults))
	for _, result := range allResults {
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if scope != "" {
		filtered := make([]qdrant.SearchResult, 0, len(results))
		for _, result := range results {
			if result.Memory.Scope == scope {
				filtered = append(filtered, result)
			}
		}
		results = filtered
	}

	if uint64(len(results)) > limit {
		results = results[:limit]
	}

	return results, nil
}

// ListMemories retrieves memories from Qdrant filtered by project and keywords,
// then applies an optional in-memory scope filter before returning.
func (s *Service) ListMemories(ctx context.Context, filterProject string, filterKeywords []string, scope string, limit uint64) ([]qdrant.Memory, error) {
	filterProject = sanitizeProject(filterProject)
	return s.qdrant.ListMemories(ctx, filterProject, filterKeywords, limit, scope)
}

// DeleteMemory removes a single memory point from Qdrant by its ID.
func (s *Service) DeleteMemory(ctx context.Context, memoryID uint64) error {
	return s.qdrant.DeleteMemory(ctx, memoryID)
}

// DeleteMemoriesByProject removes all memory points associated with the given project.
func (s *Service) DeleteMemoriesByProject(ctx context.Context, project string) error {
	project = sanitizeProject(project)
	return s.qdrant.DeleteMemoriesByProject(ctx, project)
}

// DeleteAllMemories wipes the entire Qdrant collection.
func (s *Service) DeleteAllMemories(ctx context.Context) error {
	return s.qdrant.DeleteAllMemories(ctx)
}

// generateID returns a random uint64 to use as a memory point ID.
func generateID() uint64 {
	return rand.Uint64()
}

// sanitizeProject extracts the last non-empty path segment from project, so agents
// that accidentally pass a full filesystem path (e.g. "/home/user/code/my-app")
// are normalized to just the project name (e.g. "my-app").
func sanitizeProject(project string) string {
	parts := strings.Split(project, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return project
}

// SetupTools registers all MCP tool handlers (create, search, list, delete) on the given server.
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

// handleCreateMemory validates input fields (content required, max 250 words, scope valid,
// project required for project scope), parses comma-separated keywords (max 5),
// and delegates to CreateMemory.
func (s *Service) handleCreateMemory(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	project, _ := input["project"].(string)
	contextStr, _ := input["context"].(string)
	content, _ := input["content"].(string)
	keywordsStr, _ := input["keywords"].(string)
	scopeStr, _ := input["scope"].(string)

	if content == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'content' is required"},
			},
		}, nil, nil
	}

	wordCount := len(strings.Fields(content))
	if wordCount > 250 {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Content must be at most 250 words"},
			},
		}, nil, nil
	}

	var keywords []string
	if keywordsStr != "" {
		rawKeywords := strings.SplitSeq(keywordsStr, ",")
		for kw := range rawKeywords {
			trimmed := strings.TrimSpace(kw)
			if trimmed != "" {
				keywords = append(keywords, trimmed)
			}
		}
		if len(keywords) > 5 {
			keywords = keywords[:5]
		}
	}

	scope := scopeStr
	if scope == "" {
		scope = qdrant.DefaultScope()
	}

	if !qdrant.ValidScope(scope) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)},
			},
		}, nil, nil
	}

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

// handleSearchMemories validates the optional scope and enforces that project is provided
// when scope is "project". Delegates to SearchMemories and formats results with similarity percentages.
func (s *Service) handleSearchMemories(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	project, _ := input["project"].(string)
	contextStr, _ := input["context"].(string)
	keywordsStr, _ := input["keywords"].(string)
	scopeStr, _ := input["scope"].(string)

	limit := uint64(10)
	if limitVal, ok := input["limit"].(float64); ok {
		limit = uint64(limitVal)
	}

	scope := scopeStr

	if scope != "" && !qdrant.ValidScope(scope) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)},
			},
		}, nil, nil
	}

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

// handleListMemories requires scope, enforces project for project-scope, rejects project
// for non-project scopes, and disallows the keywords parameter. Delegates to ListMemories.
func (s *Service) handleListMemories(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	project, _ := input["project"].(string)
	keywordsStr, _ := input["keywords"].(string)
	scopeStr, _ := input["scope"].(string)

	if scopeStr == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'scope' is required for list operation"},
			},
		}, nil, nil
	}

	scope := scopeStr

	if !qdrant.ValidScope(scope) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)},
			},
		}, nil, nil
	}

	if scope == qdrant.ScopeProject && project == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'project' is required when scope is 'project'"},
			},
		}, nil, nil
	}

	if scope != qdrant.ScopeProject && project != "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Parameter 'project' is only allowed when scope is 'project'"},
			},
		}, nil, nil
	}

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
		limit = min(limit, 100)
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

// handleDeleteMemory parses and validates the memory_id string parameter, then delegates to DeleteMemory.
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

// handleDeleteMemoriesByProject validates that project is provided, then delegates to DeleteMemoriesByProject.
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

// handleDeleteAllMemories delegates to DeleteAllMemories and returns a success confirmation.
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
