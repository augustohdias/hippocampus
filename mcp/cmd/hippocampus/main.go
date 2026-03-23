package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"

	"github.com/augustohdias/hippocampus/mcp/internal/config"
	hippocampus_mcp "github.com/augustohdias/hippocampus/mcp/internal/mcp"
	"github.com/augustohdias/hippocampus/mcp/internal/qdrant"
	mcp_sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MemoryResponse HTTP API Response types
type MemoryResponse struct {
	ID       uint64   `json:"id"`
	Project  string   `json:"project"`
	Context  string   `json:"context"`
	Content  string   `json:"content"`
	Keywords []string `json:"keywords"`
	Scope    string   `json:"scope,omitempty"`
}

type SearchResponse struct {
	Memory MemoryResponse `json:"memory"`
	Score  float32        `json:"score"`
}

type ListResponse struct {
	Memories []MemoryResponse `json:"memories"`
	Total    int              `json:"total"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// tryStartServer attempts to start HTTP server on a range of ports
// starting from startPort, trying up to maxAttempts ports.
// Returns an open net.Listener on the available port, or error if all attempts fail.
func tryStartServer(startPort string, maxAttempts int) (net.Listener, error) {
	port, err := strconv.Atoi(startPort)
	if err != nil {
		return nil, fmt.Errorf("invalid port %s: %v", startPort, err)
	}

	for i := range maxAttempts {
		currentPort := strconv.Itoa(port + i)
		addr := ":" + currentPort

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("Port %s busy, trying next...", currentPort)
			continue
		}

		// Return the open listener to avoid TOCTOU race condition
		return listener, nil
	}

	return nil, fmt.Errorf("could not find available port in range %d-%d", port, port+maxAttempts-1)
}

func setupHTTPServer(service *hippocampus_mcp.Service, listener net.Listener, errorChan chan error) {
	// Health endpoint: GET /api/health
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"version": "1.0.0",
			"service": "hippocampus",
		})
	})

	// Who endpoint: GET /api/who - Simple identification endpoint
	http.HandleFunc("/api/who", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"service": "hippocampus",
			"version": "1.0.0",
			"message": "hippocampus memory server",
		})
	})

	// Root endpoint: GET /
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Not found"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"service": "hippocampus",
			"version": "1.0.0",
			"endpoints": map[string]string{
				"health": "/api/health",
				"who":    "/api/who",
				"search": "/api/search?project=PROJECT&context=CONTEXT&keywords=KEYWORDS&limit=LIMIT",
				"list":   "/api/list?project=PROJECT&keywords=KEYWORDS&limit=LIMIT",
			},
		})
	})

	// Search endpoint: GET /api/search?project=X&context=Y&keywords=Z&limit=N
	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		project := r.URL.Query().Get("project")
		contextStr := r.URL.Query().Get("context")
		keywords := r.URL.Query().Get("keywords")
		scope := r.URL.Query().Get("scope")
		limit := 10

		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}

		// Validate scope if provided
		if scope != "" && !qdrant.ValidScope(scope) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)})
			return
		}

		// If scope is "project", project parameter is required
		if scope == qdrant.ScopeProject && project == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "project parameter is required when scope is 'project'"})
			return
		}

		searchCtx := context.Background()
		results, err := service.SearchMemories(searchCtx, project, contextStr, keywords, scope, uint64(limit))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
			return
		}

		response := make([]SearchResponse, len(results))
		for i, result := range results {
			response[i] = SearchResponse{
				Memory: MemoryResponse{
					ID:       result.Memory.ID,
					Project:  result.Memory.Project,
					Context:  result.Memory.Context,
					Content:  result.Memory.Content,
					Keywords: result.Memory.Keywords,
					Scope:    result.Memory.Scope,
				},
				Score: result.Similarity,
			}
		}

		json.NewEncoder(w).Encode(map[string]any{
			"results": response,
			"total":   len(response),
		})
	})

	// List endpoint: GET /api/list?project=X&keywords=Y&limit=N
	http.HandleFunc("/api/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		project := r.URL.Query().Get("project")
		keywords := r.URL.Query().Get("keywords")
		scope := r.URL.Query().Get("scope")
		limit := 50

		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}

		// Scope is required for list operation
		if scope == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "scope parameter is required for list operation"})
			return
		}

		// Validate scope
		if !qdrant.ValidScope(scope) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)})
			return
		}

		// If scope is "project", project parameter is required
		if scope == qdrant.ScopeProject && project == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "project parameter is required when scope is 'project'"})
			return
		}

		// If scope is not "project", project parameter should not be provided
		if scope != qdrant.ScopeProject && project != "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "project parameter is only allowed when scope is 'project'"})
			return
		}

		// Keywords parameter is not allowed for list operation
		if keywords != "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "keywords parameter is not allowed for list operation"})
			return
		}

		ctx := context.Background()
		memories, err := service.ListMemories(ctx, project, nil, scope, uint64(limit))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
			return
		}

		response := make([]MemoryResponse, len(memories))
		for i, m := range memories {
			response[i] = MemoryResponse{
				ID:       m.ID,
				Project:  m.Project,
				Context:  m.Context,
				Content:  m.Content,
				Keywords: m.Keywords,
				Scope:    m.Scope,
			}
		}

		json.NewEncoder(w).Encode(ListResponse{
			Memories: response,
			Total:    len(response),
		})
	})

	// Create endpoint: POST /api/create
	http.HandleFunc("/api/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Project  string   `json:"project"`
			Context  string   `json:"context"`
			Content  string   `json:"content"`
			Keywords []string `json:"keywords"`
			Scope    string   `json:"scope"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("Invalid JSON: %v", err)})
			return
		}

		if req.Content == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "content is required"})
			return
		}

		// Default scope to "project"
		if req.Scope == "" {
			req.Scope = qdrant.DefaultScope()
		}

		// Validate scope
		if !qdrant.ValidScope(req.Scope) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("Invalid scope '%s'. Must be one of: '%s', '%s', '%s'", req.Scope, qdrant.ScopeProject, qdrant.ScopePersonal, qdrant.ScopeGlobal)})
			return
		}

		// Project is required only for project scope; ignored for global/personal
		if req.Scope == qdrant.ScopeProject && req.Project == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "project is required when scope is 'project'"})
			return
		}
		if req.Scope != qdrant.ScopeProject {
			req.Project = ""
		}

		ctx := context.Background()
		memoryID, err := service.CreateMemory(ctx, req.Project, req.Context, req.Content, req.Keywords, req.Scope)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"id":      fmt.Sprintf("%d", memoryID),
			"success": true,
			"message": fmt.Sprintf("Memory created with ID %d and scope %s", memoryID, req.Scope),
		})
	})

	// Delete endpoint: DELETE /api/delete
	http.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var req struct {
			MemoryID string `json:"memory_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("Invalid JSON: %v", err)})
			return
		}
		memoryID, err := strconv.ParseUint(req.MemoryID, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("Invalid memory_id: %v", err)})
			return
		}

		ctx := context.Background()
		if err := service.DeleteMemory(ctx, memoryID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": fmt.Sprintf("Memory %d deleted successfully", memoryID),
		})
	})

	http.HandleFunc("/api/deleteAll", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "Method not allowed"})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		ctx := context.Background()
		if err := service.DeleteAllMemories(ctx); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": fmt.Sprintln("Memories deleted successfully"),
		})
	})

	port := listener.Addr().(*net.TCPAddr).Port
	log.Printf("HTTP API server listening on port %d", port)
	go func() {
		if err := http.Serve(listener, nil); err != nil {
			log.Printf("HTTP server error: %v", err)
			if errorChan != nil {
				errorChan <- err
			}
		}
	}()
}

func main() {
	// Recover from any panics and log them
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC recovered: %v", r)
			// Print stack trace
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	// Parse command line flags
	var (
		httpMode bool
		httpPort string
		showHelp bool
	)

	flag.BoolVar(&httpMode, "http", false, "Run in HTTP-only mode (no MCP stdio)")
	flag.StringVar(&httpPort, "port", "", "HTTP server port (default: 8765 or HIPPOCAMPUS_HTTP_PORT env)")
	flag.BoolVar(&showHelp, "help", false, "Show usage help")
	flag.Parse()

	// Check which flags were explicitly set
	portFlagExplicitlySet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			portFlagExplicitlySet = true
		}
	})

	if showHelp {
		fmt.Println("Hippocampus MCP Server")
		fmt.Println("Usage: hippocampus [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  hippocampus              # Run MCP server with HTTP API")
		fmt.Println("  hippocampus --http      # Run only HTTP API server")
		fmt.Println("  hippocampus --http --port 8888")
		return
	}

	log.Println("Starting Hippocampus MCP server...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if cfg.OllamaHost != "" {
		log.Printf("Configuration loaded: Ollama=%s (host: %s), Qdrant=%s", cfg.OllamaModel, cfg.OllamaHost, cfg.QdrantHost)
	} else {
		log.Printf("Configuration loaded: Ollama=%s, Qdrant=%s", cfg.OllamaModel, cfg.QdrantHost)
	}

	service, err := hippocampus_mcp.NewService(cfg)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}
	log.Println("Service created successfully")
	defer service.Close()

	ctx := context.Background()

	log.Println("Ensuring Qdrant collection exists...")
	if err := service.EnsureCollection(ctx); err != nil {
		log.Printf("Warning: failed to ensure collection (might be ok if already exists): %v", err)
	} else {
		log.Println("Collection ensured successfully")
	}

	// Start HTTP API server
	// Determine HTTP port: flag -> env -> default
	if httpPort == "" {
		httpPort = os.Getenv("HIPPOCAMPUS_HTTP_PORT")
		if httpPort == "" {
			httpPort = "8765"
		}
	}

	// Check if we should run HTTP only (for background daemon)
	// Support both flag and env var for compatibility
	httpOnlyEnv := os.Getenv("HIPPOCAMPUS_HTTP_ONLY")
	runHTTPOnly := httpMode || httpOnlyEnv == "true" || httpOnlyEnv == "1"

	// Create error channel for HTTP server (only used in HTTP-only mode)
	if runHTTPOnly {
		var listener net.Listener
		var err error

		// Try to find an available port (for auto-start scenarios)
		// If user explicitly specified a port via flag or env, we should respect it
		// and not try alternatives (they might want that specific port)
		originalPort := httpPort
		userSpecifiedPort := portFlagExplicitlySet || os.Getenv("HIPPOCAMPUS_HTTP_PORT") != ""
		if !userSpecifiedPort {
			// For auto-start, try to find available port starting from default
			listener, err = tryStartServer(httpPort, 10) // Try up to 10 ports
			if err != nil {
				log.Fatalf("Failed to find available port: %v", err)
			}
			actualPort := listener.Addr().(*net.TCPAddr).Port
			if originalPort != strconv.Itoa(actualPort) {
				log.Printf("Port %s busy, using port %d instead", originalPort, actualPort)
			}
		} else {
			// User specified port explicitly, create listener directly
			listener, err = net.Listen("tcp", ":"+httpPort)
			if err != nil {
				log.Fatalf("Failed to listen on port %s: %v", httpPort, err)
			}
		}
		httpErrorChan := make(chan error, 1)
		actualPort := listener.Addr().(*net.TCPAddr).Port
		log.Printf("HTTP API available at http://localhost:%d/api/", actualPort)
		setupHTTPServer(service, listener, httpErrorChan)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case err := <-httpErrorChan:
			log.Fatalf("HTTP server failed to start: %v", err)
		case <-sigChan:
			log.Println("Shutting down...")
			return
		}
	} else {
		log.Println("Hippocampus MCP server starting...")
		log.Printf("Using Ollama model: %s", cfg.OllamaModel)
		log.Printf("Using Qdrant collection: %s", cfg.QdrantCollection)
		// Otherwise run full MCP server
		server := mcp_sdk.NewServer(&mcp_sdk.Implementation{
			Name:    "hippocampus",
			Version: "1.0.0",
		}, nil)

		if err := service.SetupTools(server); err != nil {
			log.Fatalf("Failed to setup tools: %v", err)
		}

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			log.Println("Shutting down...")
			os.Exit(0)
		}()

		if err := server.Run(ctx, &mcp_sdk.StdioTransport{}); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}
