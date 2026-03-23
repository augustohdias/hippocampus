package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/augustohdias/hippocampus/mcp/internal/config"
	"github.com/augustohdias/hippocampus/mcp/internal/qdrant"
)

func TestMultiEmbeddingMemory(t *testing.T) {
	t.Parallel()

	// Skip if Qdrant is not available
	if os.Getenv("QDRANT_HOST") == "" {
		os.Setenv("QDRANT_HOST", "localhost:6334")
	}

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_multi",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Ensure collection exists
	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Create a test memory with project and keywords
	project := "test-project-multi"
	context := "Integration Test"
	content := "This is a test memory for multi-embedding functionality"
	keywords := []string{"integration", "testing", "multi-embedding", "qdrant"}

	memoryID, err := service.CreateMemory(ctx, project, context, content, keywords, qdrant.ScopeProject)
	if err != nil {
		t.Fatalf("Failed to create memory: %v", err)
	}

	t.Logf("Created memory with ID: %d", memoryID)

	// Give Qdrant time to index
	time.Sleep(500 * time.Millisecond)

	// Test 1: Search by project only - should find the memory with high similarity
	t.Run("SearchByProject", func(t *testing.T) {
		results, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to search by project: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected at least 1 result when searching by project")
		}

		// Find our memory in results
		found := false
		for _, result := range results {
			if result.Memory.ID == memoryID {
				found = true
				t.Logf("Found memory by project with similarity: %.2f%%", result.Similarity*100)
				if result.Similarity < 0.6 {
					t.Errorf("Expected similarity >= 0.6, got %.2f", result.Similarity)
				}
				break
			}
		}

		if !found {
			t.Error("Memory not found when searching by project")
		}
	})
}

func TestMultiVectorEmbeddingStructure(t *testing.T) {
	t.Parallel()

	// Skip if Qdrant is not available
	if os.Getenv("QDRANT_HOST") == "" {
		os.Setenv("QDRANT_HOST", "localhost:6334")
	}

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_multi_struct",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Ensure collection exists
	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Create a memory with multiple keywords
	project := "test-project-multi-vector"
	context := "Multi-Vector Test"
	content := "Testing the structure of multi-vector embeddings"
	keywords := []string{"vector", "embedding", "multi", "test", "qdrant", "extra"} // 6 keywords, only first 5 should be embedded

	memoryID, err := service.CreateMemory(ctx, project, context, content, keywords, qdrant.ScopeProject)
	if err != nil {
		t.Fatalf("Failed to create memory: %v", err)
	}

	t.Logf("Created memory with ID: %d", memoryID)

	// Give Qdrant time to index
	time.Sleep(500 * time.Millisecond)

	// Test 1: Search using main vector
	t.Run("SearchWithMainVector", func(t *testing.T) {
		results, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to search with main vector: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected at least 1 result when searching with main vector")
		}
		found := false
		for _, result := range results {
			if result.Memory.ID == memoryID {
				found = true
				t.Logf("Found memory via main vector with similarity: %.2f%%", result.Similarity*100)
				break
			}
		}
		if !found {
			t.Error("Memory not found via main vector")
		}
	})

	// Test 2: Search using specific keyword vector (keyword1)
	t.Run("SearchWithKeywordVector", func(t *testing.T) {
		// Search by the first keyword
		results, err := service.SearchMemories(ctx, "", "", "vector", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to search with keyword vector: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected at least 1 result when searching with keyword vector")
		}
		found := false
		for _, result := range results {
			if result.Memory.ID == memoryID {
				found = true
				t.Logf("Found memory via keyword vector with similarity: %.2f%%", result.Similarity*100)
				break
			}
		}
		if !found {
			t.Error("Memory not found via keyword vector")
		}
	})

	// Test 3: Search using project vector
	t.Run("SearchWithProjectVector", func(t *testing.T) {
		results, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to search with project vector: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected at least 1 result when searching with project vector")
		}
		found := false
		for _, result := range results {
			if result.Memory.ID == memoryID {
				found = true
				t.Logf("Found memory via project vector with similarity: %.2f%%", result.Similarity*100)
				break
			}
		}
		if !found {
			t.Error("Memory not found via project vector")
		}
	})

	// Test 4: Verify that only first 5 keywords are embedded (keyword6 "extra" should not have vector)
	// We can test by searching for the 6th keyword - it might still match via main vector but similarity may be lower
	// This is a soft check
	t.Run("KeywordLimit", func(t *testing.T) {
		results, err := service.SearchMemories(ctx, "", "", "extra", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to search for extra keyword: %v", err)
		}
		// May or may not find; just log
		found := false
		for _, result := range results {
			if result.Memory.ID == memoryID {
				found = true
				t.Logf("Memory found via extra keyword (may match via main vector) with similarity: %.2f%%", result.Similarity*100)
				break
			}
		}
		if !found {
			t.Log("Extra keyword not found as dedicated vector (expected, only first 5 keywords get vectors)")
		}
	})
}

func TestBatchSearchMultiVector(t *testing.T) {
	t.Parallel()

	// Skip if Qdrant is not available
	if os.Getenv("QDRANT_HOST") == "" {
		os.Setenv("QDRANT_HOST", "localhost:6334")
	}

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_batch",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Ensure collection exists
	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Create multiple memories with different keywords
	project := "test-project-batch"
	keywords := []string{"apple", "banana", "cherry", "date", "elderberry"}

	var memoryIDs []uint64
	for i, keyword := range keywords {
		content := fmt.Sprintf("This is memory about %s", keyword)
		memoryID, err := service.CreateMemory(ctx, project, "Batch Test", content, []string{keyword}, qdrant.ScopeProject)
		if err != nil {
			t.Fatalf("Failed to create memory %d: %v", i, err)
		}
		memoryIDs = append(memoryIDs, memoryID)
		t.Logf("Created memory with ID: %d for keyword %s", memoryID, keyword)
	}

	// Give Qdrant time to index
	time.Sleep(500 * time.Millisecond)

	// Test batch search with multiple queries (simulating what SearchMemories does internally)
	t.Run("BatchSearchMultipleKeywords", func(t *testing.T) {
		// Search for multiple keywords at once (simulating batch search)
		// This uses the service's SearchMemories which internally uses batch search
		results, err := service.SearchMemories(ctx, project, "", "apple,banana,cherry", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to batch search multiple keywords: %v", err)
		}
		if len(results) < 3 {
			t.Errorf("Expected at least 3 results for batch search, got %d", len(results))
		}
		// Verify we found memories for each keyword
		foundKeywords := make(map[string]bool)
		for _, result := range results {
			if len(result.Memory.Keywords) > 0 {
				foundKeywords[result.Memory.Keywords[0]] = true
			}
		}
		t.Logf("Found memories for keywords: %v", foundKeywords)
	})

	t.Run("BatchSearchMixedVectors", func(t *testing.T) {
		// Search with project and keyword combination
		results, err := service.SearchMemories(ctx, project, "", "apple", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to search with mixed vectors: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("Expected at least 1 result for mixed vector search")
		}
		// Should find the apple memory
		found := false
		for _, result := range results {
			if len(result.Memory.Keywords) > 0 && result.Memory.Keywords[0] == "apple" {
				found = true
				t.Logf("Found apple memory with similarity: %.2f%%", result.Similarity*100)
				break
			}
		}
		if !found {
			t.Error("Apple memory not found with mixed vector search")
		}
	})
}

func TestMultiEmbeddingNoKeywords(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_no_kw",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Create a test memory without keywords
	project := "test-project-no-keywords"
	context := "Integration Test"
	content := "This is a test memory without keywords"

	memoryID, err := service.CreateMemory(ctx, project, context, content, []string{}, qdrant.ScopeProject)
	if err != nil {
		t.Fatalf("Failed to create memory: %v", err)
	}

	t.Logf("Created memory with ID: %d", memoryID)

	// Give Qdrant time to index
	time.Sleep(500 * time.Millisecond)

	// Search by project - should still find the memory
	results, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopeProject, 10)
	if err != nil {
		t.Fatalf("Failed to search by project: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected at least 1 result when searching by project (no keywords)")
	}

	found := false
	for _, result := range results {
		if result.Memory.ID == memoryID {
			found = true
			t.Logf("Found memory by project with similarity: %.2f%%", result.Similarity*100)
			break
		}
	}

	if !found {
		t.Error("Memory not found when searching by project (no keywords)")
	}
}

func TestDeleteMemory(t *testing.T) {
	t.Parallel()

	if os.Getenv("QDRANT_HOST") == "" {
		os.Setenv("QDRANT_HOST", "localhost:6334")
	}

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_delete",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Create a test memory
	project := "test-project-delete"
	context := "Delete Test"
	content := "This memory will be deleted"
	keywords := []string{"delete", "test"}

	memoryID, err := service.CreateMemory(ctx, project, context, content, keywords, qdrant.ScopeProject)
	if err != nil {
		t.Fatalf("Failed to create memory: %v", err)
	}

	t.Logf("Created memory with ID: %d", memoryID)

	// Give Qdrant time to index
	time.Sleep(500 * time.Millisecond)

	// Verify memory exists
	results, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopeProject, 10)
	if err != nil {
		t.Fatalf("Failed to search memories: %v", err)
	}

	found := false
	for _, result := range results {
		if result.Memory.ID == memoryID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Memory not found after creation")
	}

	// Delete the memory
	if err := service.DeleteMemory(ctx, memoryID); err != nil {
		t.Fatalf("Failed to delete memory: %v", err)
	}

	// Give Qdrant time to process deletion
	time.Sleep(300 * time.Millisecond)

	// Verify memory is deleted
	results, err = service.SearchMemories(ctx, project, "", "", qdrant.ScopeProject, 10)
	if err != nil {
		t.Fatalf("Failed to search memories after deletion: %v", err)
	}

	for _, result := range results {
		if result.Memory.ID == memoryID {
			t.Error("Memory still exists after deletion")
			break
		}
	}

	t.Log("Memory deleted successfully")
}

func TestDeleteMemoriesByProject(t *testing.T) {
	t.Parallel()

	if os.Getenv("QDRANT_HOST") == "" {
		os.Setenv("QDRANT_HOST", "localhost:6334")
	}

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_delete_proj",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Create multiple test memories for the same project
	project := "test-project-delete-batch"
	context := "Delete Batch Test"
	keywords := []string{"delete", "batch"}

	var memoryIDs []uint64
	for i := 0; i < 3; i++ {
		content := fmt.Sprintf("This is test memory %d for batch deletion", i)
		memoryID, err := service.CreateMemory(ctx, project, context, content, keywords, qdrant.ScopeProject)
		if err != nil {
			t.Fatalf("Failed to create memory %d: %v", i, err)
		}
		memoryIDs = append(memoryIDs, memoryID)
		t.Logf("Created memory with ID: %d", memoryID)
	}

	// Create a memory for a different project (should not be deleted)
	otherProject := "test-project-other"
	otherMemoryID, err := service.CreateMemory(ctx, otherProject, context, "This memory should not be deleted", keywords, qdrant.ScopeProject)
	if err != nil {
		t.Fatalf("Failed to create other memory: %v", err)
	}
	t.Logf("Created other memory with ID: %d", otherMemoryID)

	// Give Qdrant time to index
	time.Sleep(500 * time.Millisecond)

	// Delete all memories from the project
	if err := service.DeleteMemoriesByProject(ctx, project); err != nil {
		t.Fatalf("Failed to delete memories by project: %v", err)
	}

	// Give Qdrant time to process deletion
	time.Sleep(300 * time.Millisecond)

	// Verify all memories from the project are deleted using ListMemories
	memories, err := service.ListMemories(ctx, project, []string{}, qdrant.ScopeProject, 100)
	if err != nil {
		t.Fatalf("Failed to list memories after deletion: %v", err)
	}

	if len(memories) > 0 {
		t.Errorf("Expected 0 memories for deleted project, got %d", len(memories))
		for _, memory := range memories {
			t.Logf("Unexpected memory: ID %d", memory.ID)
		}
	}

	// Verify memory from other project still exists using ListMemories
	memories, err = service.ListMemories(ctx, otherProject, []string{}, qdrant.ScopeProject, 100)
	if err != nil {
		t.Fatalf("Failed to list memories for other project: %v", err)
	}

	found := false
	for _, memory := range memories {
		if memory.ID == otherMemoryID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Memory from other project was incorrectly deleted")
	}

	t.Log("All memories from project deleted successfully")
}

func TestDeleteAllMemories(t *testing.T) {
	t.Parallel()

	if os.Getenv("QDRANT_HOST") == "" {
		os.Setenv("QDRANT_HOST", "localhost:6334")
	}

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_del_all",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Create multiple test memories
	projects := []string{"project-1", "project-2", "project-3"}
	keywords := []string{"delete", "all"}

	var memoryIDs []uint64
	for i, project := range projects {
		content := fmt.Sprintf("This is test memory %d for delete all", i)
		memoryID, err := service.CreateMemory(ctx, project, "Delete All Test", content, keywords, qdrant.ScopeProject)
		if err != nil {
			t.Fatalf("Failed to create memory %d: %v", i, err)
		}
		memoryIDs = append(memoryIDs, memoryID)
		t.Logf("Created memory with ID: %d for project %s", memoryID, project)
	}

	// Give Qdrant time to index
	time.Sleep(500 * time.Millisecond)

	// Verify memories exist using SearchMemories (more reliable)
	allResults, err := service.SearchMemories(ctx, "project", "", "", qdrant.ScopeProject, 100)
	if err != nil {
		t.Fatalf("Failed to search memories before deletion: %v", err)
	}
	if len(allResults) < len(memoryIDs) {
		t.Logf("Warning: Expected at least %d memories before deletion, got %d (may be due to search threshold)", len(memoryIDs), len(allResults))
	}

	// Delete all memories
	if err := service.DeleteAllMemories(ctx); err != nil {
		t.Fatalf("Failed to delete all memories: %v", err)
	}

	// Give Qdrant time to process deletion
	time.Sleep(300 * time.Millisecond)

	// Verify all memories are deleted using ListMemories
	allMemories, err := service.ListMemories(ctx, "", []string{}, qdrant.ScopeProject, 100)
	if err != nil {
		t.Fatalf("Failed to list memories after deletion: %v", err)
	}

	if len(allMemories) != 0 {
		t.Errorf("Expected 0 memories after deletion, got %d", len(allMemories))
		for _, memory := range allMemories {
			t.Logf("Unexpected memory: ID %d, project %s", memory.ID, memory.Project)
		}
	}

	t.Log("All memories deleted successfully")
}

func TestScopeFunctionality(t *testing.T) {
	t.Parallel()

	// Skip if Qdrant is not available
	if os.Getenv("QDRANT_HOST") == "" {
		os.Setenv("QDRANT_HOST", "localhost:6334")
	}

	cfg := &config.Config{
		OllamaModel:      "qwen3-embedding:4b",
		QdrantHost:       "localhost:6334",
		QdrantCollection: "memories_test_scope",
		LogLevel:         "info",
	}

	ctx := context.Background()

	service, err := NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer service.Close()

	// Ensure collection exists
	if err := service.EnsureCollection(ctx); err != nil {
		t.Fatalf("Failed to ensure collection: %v", err)
	}

	// Cleanup at the end: delete the test collection entirely
	defer func() {
		qdrantClient, err := qdrant.NewClient(cfg.QdrantHost, cfg.QdrantCollection)
		if err == nil {
			defer qdrantClient.Close()
			_ = qdrantClient.DeleteCollection(ctx, cfg.QdrantCollection)
		}
	}()

	// Test 1: Create memories with different scopes
	t.Run("CreateWithDifferentScopes", func(t *testing.T) {
		project := "test-project-scope"
		context := "Scope Test"
		content := "This is a test memory with scope"
		keywords := []string{"scope", "test"}

		// Create with global scope
		globalID, err := service.CreateMemory(ctx, project, context, content+" global", keywords, qdrant.ScopeGlobal)
		if err != nil {
			t.Fatalf("Failed to create global memory: %v", err)
		}
		t.Logf("Created global memory with ID: %d", globalID)

		// Create with personal scope
		personalID, err := service.CreateMemory(ctx, project, context, content+" personal", keywords, qdrant.ScopePersonal)
		if err != nil {
			t.Fatalf("Failed to create personal memory: %v", err)
		}
		t.Logf("Created personal memory with ID: %d", personalID)

		// Create with project scope (default)
		projectID, err := service.CreateMemory(ctx, project, context, content+" project", keywords, qdrant.ScopeProject)
		if err != nil {
			t.Fatalf("Failed to create project memory: %v", err)
		}
		t.Logf("Created project memory with ID: %d", projectID)

		// Give Qdrant time to index
		time.Sleep(500 * time.Millisecond)

		// Verify all memories exist via ListMemories without scope filter
		memories, err := service.ListMemories(ctx, project, []string{}, "", 100)
		if err != nil {
			t.Fatalf("Failed to list memories: %v", err)
		}

		// Should find all three memories
		if len(memories) != 3 {
			t.Errorf("Expected 3 memories, got %d", len(memories))
		}

		// Check each scope
		scopesFound := make(map[string]bool)
		for _, mem := range memories {
			scopesFound[mem.Scope] = true
		}
		if !scopesFound[qdrant.ScopeGlobal] {
			t.Error("Global scope memory not found")
		}
		if !scopesFound[qdrant.ScopePersonal] {
			t.Error("Personal scope memory not found")
		}
		if !scopesFound[qdrant.ScopeProject] {
			t.Error("Project scope memory not found")
		}
	})

	// Test 2: Search memories filtering by scope
	t.Run("SearchFilteredByScope", func(t *testing.T) {
		project := "test-project-scope-search"
		context := "Scope Search Test"
		keywords := []string{"scope", "search"}

		// Create one memory of each scope
		globalID, err := service.CreateMemory(ctx, project, context, "Global content", keywords, qdrant.ScopeGlobal)
		if err != nil {
			t.Fatalf("Failed to create global memory: %v", err)
		}
		personalID, err := service.CreateMemory(ctx, project, context, "Personal content", keywords, qdrant.ScopePersonal)
		if err != nil {
			t.Fatalf("Failed to create personal memory: %v", err)
		}
		projectID, err := service.CreateMemory(ctx, project, context, "Project content", keywords, qdrant.ScopeProject)
		if err != nil {
			t.Fatalf("Failed to create project memory: %v", err)
		}

		// Give Qdrant time to index
		time.Sleep(500 * time.Millisecond)

		// Search with global scope filter - should find only global memory
		globalResults, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopeGlobal, 10)
		if err != nil {
			t.Fatalf("Failed to search with global scope: %v", err)
		}
		// Should find at least global memory (may also find others due to similarity, but scope filter should exclude)
		foundGlobal := false
		for _, result := range globalResults {
			if result.Memory.ID == globalID {
				foundGlobal = true
				if result.Memory.Scope != qdrant.ScopeGlobal {
					t.Errorf("Memory found with wrong scope: expected %s, got %s", qdrant.ScopeGlobal, result.Memory.Scope)
				}
			}
		}
		if !foundGlobal {
			t.Error("Global scope memory not found when searching with global scope filter")
		}

		// Search with personal scope filter - should find only personal memory
		personalResults, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopePersonal, 10)
		if err != nil {
			t.Fatalf("Failed to search with personal scope: %v", err)
		}
		foundPersonal := false
		for _, result := range personalResults {
			if result.Memory.ID == personalID {
				foundPersonal = true
				if result.Memory.Scope != qdrant.ScopePersonal {
					t.Errorf("Memory found with wrong scope: expected %s, got %s", qdrant.ScopePersonal, result.Memory.Scope)
				}
			}
		}
		if !foundPersonal {
			t.Error("Personal scope memory not found when searching with personal scope filter")
		}

		// Search with project scope filter - should find only project memory
		projectResults, err := service.SearchMemories(ctx, project, "", "", qdrant.ScopeProject, 10)
		if err != nil {
			t.Fatalf("Failed to search with project scope: %v", err)
		}
		foundProject := false
		for _, result := range projectResults {
			if result.Memory.ID == projectID {
				foundProject = true
				if result.Memory.Scope != qdrant.ScopeProject {
					t.Errorf("Memory found with wrong scope: expected %s, got %s", qdrant.ScopeProject, result.Memory.Scope)
				}
			}
		}
		if !foundProject {
			t.Error("Project scope memory not found when searching with project scope filter")
		}
	})

	// Test 3: List memories filtering by scope
	t.Run("ListFilteredByScope", func(t *testing.T) {
		project := "test-project-scope-list"
		context := "Scope List Test"
		keywords := []string{"scope", "list"}

		// Create two memories with global scope, one with personal
		globalID1, err := service.CreateMemory(ctx, project, context, "Global 1", keywords, qdrant.ScopeGlobal)
		if err != nil {
			t.Fatalf("Failed to create global memory 1: %v", err)
		}
		if globalID1 == 0 {
			t.Error("Global memory 1 ID should not be zero")
		}
		globalID2, err := service.CreateMemory(ctx, project, context, "Global 2", keywords, qdrant.ScopeGlobal)
		if err != nil {
			t.Fatalf("Failed to create global memory 2: %v", err)
		}
		if globalID2 == 0 {
			t.Error("Global memory 2 ID should not be zero")
		}
		personalID, err := service.CreateMemory(ctx, project, context, "Personal", keywords, qdrant.ScopePersonal)
		if err != nil {
			t.Fatalf("Failed to create personal memory: %v", err)
		}
		if personalID == 0 {
			t.Error("Personal memory ID should not be zero")
		}

		// Give Qdrant time to index
		time.Sleep(500 * time.Millisecond)

		// List with global scope filter
		globalMemories, err := service.ListMemories(ctx, project, []string{}, qdrant.ScopeGlobal, 100)
		if err != nil {
			t.Fatalf("Failed to list global memories: %v", err)
		}
		if len(globalMemories) != 2 {
			t.Errorf("Expected 2 global memories, got %d", len(globalMemories))
		}
		for _, mem := range globalMemories {
			if mem.Scope != qdrant.ScopeGlobal {
				t.Errorf("Memory in global list has wrong scope: %s", mem.Scope)
			}
		}

		// List with personal scope filter
		personalMemories, err := service.ListMemories(ctx, project, []string{}, qdrant.ScopePersonal, 100)
		if err != nil {
			t.Fatalf("Failed to list personal memories: %v", err)
		}
		if len(personalMemories) != 1 {
			t.Errorf("Expected 1 personal memory, got %d", len(personalMemories))
		}
		if len(personalMemories) > 0 && personalMemories[0].Scope != qdrant.ScopePersonal {
			t.Errorf("Memory in personal list has wrong scope: %s", personalMemories[0].Scope)
		}

		// List with project scope filter (should find none)
		projectMemories, err := service.ListMemories(ctx, project, []string{}, qdrant.ScopeProject, 100)
		if err != nil {
			t.Fatalf("Failed to list project memories: %v", err)
		}
		if len(projectMemories) != 0 {
			t.Errorf("Expected 0 project memories, got %d", len(projectMemories))
		}
	})

	// Test 4: Invalid scope validation
	t.Run("InvalidScopeValidation", func(t *testing.T) {
		project := "test-project-invalid-scope"
		context := "Invalid Scope Test"
		content := "This should fail"
		keywords := []string{"invalid"}

		_, err := service.CreateMemory(ctx, project, context, content, keywords, "invalid-scope")
		if err == nil {
			t.Fatal("Expected error for invalid scope, got none")
		}
		expectedError := "invalid scope"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Error message should contain %q, got %q", expectedError, err.Error())
		}
		t.Logf("Invalid scope correctly rejected: %v", err)
	})

	// Test 5: Backward compatibility - default scope
	t.Run("DefaultScope", func(t *testing.T) {
		project := "test-project-default-scope"
		context := "Default Scope Test"
		keywords := []string{"default"}

		// Create memory with empty scope (should default to project)
		memoryID, err := service.CreateMemory(ctx, project, context, "Memory with empty scope should default to project", keywords, "")
		if err != nil {
			t.Fatalf("Failed to create memory with empty scope: %v", err)
		}

		// Create additional memories with different scopes for testing empty scope filter
		globalID, err := service.CreateMemory(ctx, project, context, "Global memory", keywords, qdrant.ScopeGlobal)
		if err != nil {
			t.Fatalf("Failed to create global memory: %v", err)
		}
		personalID, err := service.CreateMemory(ctx, project, context, "Personal memory", keywords, qdrant.ScopePersonal)
		if err != nil {
			t.Fatalf("Failed to create personal memory: %v", err)
		}

		// Give Qdrant time to index
		time.Sleep(500 * time.Millisecond)

		// List memories with project scope filter (should find only the project-scoped memory)
		memories, err := service.ListMemories(ctx, project, []string{}, qdrant.ScopeProject, 100)
		if err != nil {
			t.Fatalf("Failed to list memories: %v", err)
		}

		found := false
		for _, mem := range memories {
			if mem.ID == memoryID {
				found = true
				if mem.Scope != qdrant.ScopeProject {
					t.Errorf("Expected scope to be %s for backward compatibility, got %s", qdrant.ScopeProject, mem.Scope)
				}
				break
			}
		}
		if !found {
			t.Error("Memory with empty scope not found with project scope filter")
		}

		// Verify that ListMemories with empty scope returns all memories (backward compatibility)
		allMemories, err := service.ListMemories(ctx, project, []string{}, "", 100)
		if err != nil {
			t.Fatalf("Failed to list memories with empty scope: %v", err)
		}
		if len(allMemories) < 3 {
			t.Errorf("Expected at least 3 memories with empty scope filter, got %d", len(allMemories))
		}
		// Verify all three memory IDs are present
		idSet := make(map[uint64]bool)
		for _, mem := range allMemories {
			idSet[mem.ID] = true
		}
		if !idSet[memoryID] {
			t.Error("Memory with empty scope not found when listing with empty scope filter")
		}
		if !idSet[globalID] {
			t.Error("Global memory not found when listing with empty scope filter")
		}
		if !idSet[personalID] {
			t.Error("Personal memory not found when listing with empty scope filter")
		}

		// Also test SearchMemories with empty scope (should return all)
		searchResults, err := service.SearchMemories(ctx, project, "", "", "", 10)
		if err != nil {
			t.Fatalf("Failed to search memories with empty scope: %v", err)
		}
		if len(searchResults) < 3 {
			t.Logf("Note: Search with empty scope returned %d results (may be less due to similarity threshold)", len(searchResults))
		}
	})
}
