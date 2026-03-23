# AGENTS.md - Hippocampus MCP Server

## Build & Run Commands

```bash
# Build
make build              # Builds to bin/hippocampus

# Install (MCP server + OpenCode plugin)
make install            # Full install: binary + plugin + OpenCode config + start Qdrant/Ollama
make install-yes        # Same as above with auto-confirm (for CI/CD)
make install-binary     # Install only the MCP binary to ~/.local/bin/
make install-plugin     # Install only the OpenCode plugin to ~/.hippocampus/plugin/

# Run
make run                # Build and run
make dev                # Full setup: docker + ollama + run

# Dependencies
go mod download         # Install Go dependencies

# Docker services
make docker-up          # Start Qdrant
make docker-down        # Stop Qdrant

# Cleanup
make clean              # Remove bin/

# Uninstall
make uninstall          # Remove binary and plugin (config preserved)
```

## Testing

```bash
make test                     # Run all tests
go test ./...                 # Run all tests (verbose)
go test ./mcp/internal/mcp/...    # Test specific package
go test -v ./...              # Verbose output
go test -run TestName ./...   # Run single test by name
go test -run TestName -v ./mcp/internal/mcp  # Single test in package
```

**Note**: Tests require Qdrant and Ollama running (default `localhost:6334`). The test suite uses `t.Parallel()` and creates separate collections for each test. Tests cover multi‑embedding memory indexing, search, and deletion.

When adding tests:

- Place `*_test.go` files alongside the code they test (e.g., `mcp/internal/mcp/service_test.go`)
- Use `github.com/stretchr/testify` for assertions (already in go.mod)
- Use table‑driven tests with `t.Parallel()` for independent cases
- Mock external dependencies (Ollama, Qdrant) when possible; integration tests use real services

## Required Services

Hippocampus requires **Qdrant** and **Ollama** to be running before use.

### Starting Services

**Qdrant** (from project directory):
```bash
# First time setup - this will also configure auto-restart on reboot
docker compose up -d qdrant

# Qdrant is configured with 'restart: unless-stopped', so after the first
# manual start, it will automatically restart on system reboot.
```

**Ollama** (in a separate terminal):
```bash
ollama serve
```

### Pulling the Embedding Model

```bash
ollama pull qwen3-embedding:4b
```

### Using External Services

To use externally hosted Qdrant or Ollama services, configure these environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `QDRANT_HOST` | `localhost:6334` | Qdrant gRPC endpoint (format: `host:port`) |
| `OLLAMA_MODEL` | `qwen3-embedding:4b` | Ollama embedding model name |

Example with external Qdrant:
```bash
export QDRANT_HOST="qdrant.example.com:6334"
hippocampus
```

Example with external Ollama:
```bash
export OLLAMA_MODEL="nomic-embed-text"
hippocampus
```

## Code Style Guidelines

### Imports

- Standard library first, then external packages
- Separate groups with blank lines (no blank line within groups)
- Use aliasing for conflicts: `mcp_sdk "github.com/modelcontextprotocol/go-sdk/mcp"`
- Internal packages: `github.com/augustohdias/hippocampus/mcp/internal/...`

### Formatting

- Run `gofmt -w .` or `go fmt ./...` before committing
- Use tabs for indentation (Go standard)
- Max line length: follow gofmt defaults

### Naming Conventions

- **Exported**: PascalCase (`Config`, `NewService`, `Memory`)
- **Unexported**: camelCase (`cfg`, `ollamaClient`, `generateID`)
- **Interfaces**: name by function if small (`Reader`, `Writer`)
- **Constants**: UPPER_SNAKE for env keys (`OLLAMA_MODEL`)
- **Files**: snake_case matching package (`client.go`, `service.go`)
- **Receivers**: short names matching type (`c`, `s`, `cl`)

### Types & Structs

- Define structs with meaningful field names
- Use pointer receivers for methods that modify state
- Use value receivers for read‑only methods
- Keep structs focused (single responsibility)

Example:

```go
type Config struct {
    OllamaModel      string
    QdrantHost       string
    QdrantCollection string
    LogLevel         string
}
```

### Error Handling

- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Return errors up the stack; log only at boundaries
- Use `errors.Is()` / `errors.As()` for error type checks
- Validate inputs early, return descriptive errors
- For CLI errors: include stderr output when available

Example:

```go
if err != nil {
    return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
}
```

### Context Usage

- Pass `context.Context` as first parameter
- Use `ctx` as variable name
- Respect cancellation and timeouts

### Logging

- Use standard `log` package (consistent with codebase)
- Log levels: info for startup, warn for non‑fatal, error for failures
- Include context in log messages: `log.Printf("Error X: %v", err)`

### Package Structure

```
mcp/
  cmd/hippocampus/    # Entry point (main package)
  internal/
    config/           # Configuration loading
    mcp/              # MCP service & tools
    ollama/           # Ollama client (CLI wrapper)
    qdrant/           # Qdrant client (gRPC)
```

### Function Patterns

- Constructor: `NewXxx()` returns pointer or interface
- Methods: receivers named `c`, `s`, `cl` matching type
- Keep functions focused (< 50 lines ideal)
- Early returns over nested conditionals

### Comments

- Doc comments for exported: `// Function does X`
- Inline comments sparingly, explain "why" not "what"
- No trailing comments

### Testing Conventions

- Table‑driven tests for multiple cases
- Use `t.Parallel()` for independent tests
- Mock external dependencies (Ollama, Qdrant)
- Test error paths, not just happy path

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_MODEL` | `qwen3-embedding:4b` | Embedding model |
| `QDRANT_HOST` | `localhost:6334` | Qdrant gRPC endpoint |
| `QDRANT_COLLECTION` | `memories` | Collection name |
| `LOG_LEVEL` | `info` | Logging level |
| `HIPPOCAMPUS_HTTP_PORT` | `8765` | HTTP API port (used in HTTP‑only mode) |
| `HIPPOCAMPUS_HTTP_ONLY` | (none) | Set to `"true"` or `"1"` to run only HTTP API, no MCP stdio |

## MCP Tools

1. `create_memory` – Store memory with embedding
   - **scope**: Optional, one of `global`, `personal`, `project` (default: `project`)
2. `search_memories` – Semantic search by similarity
   - **project**: Optional, required when `scope="project"`
   - **scope**: Optional, filter results by scope (`global`, `personal`, `project`). No default – search across all scopes if omitted.
   - **context**, **keywords**: Optional search filters
   - **limit**: Optional, default 10
3. `list_memories` – List memories by scope
   - **scope**: Required, one of `global`, `personal`, `project`
   - **project**: Required when `scope="project"`, not allowed for other scopes
   - **limit**: Optional, default 50, max 100
   - **keywords**: Not allowed
4. `delete_memory` – Delete a specific memory by ID
5. `delete_memories_by_project` – Delete all memories from a project
6. `delete_all_memories` – Wipe entire collection

**Note**: Multi‑embedding memory indexing is implemented – each memory stores multiple embeddings (main, project‑only, first three keywords individually) for better search recall.

### Scope Types

- **global**: Memory accessible from any project/user
- **personal**: Memory accessible only by the current user (not yet implemented user identification)
- **project**: Memory accessible only within a specific project (default, backward compatible)

Memories created before scope support are treated as `project` scope.

### Examples

**Creating a global memory**:
```json
{
  "project": "myapp",
  "context": "API Keys",
  "content": "Stripe secret key: sk_live_...",
  "keywords": "stripe,api,keys",
  "scope": "global"
}
```

**Searching for personal memories** (project not required for personal scope):
```json
{
  "keywords": "password",
  "scope": "personal"
}
```

**Searching across all scopes** (no scope filter):
```json
{
  "keywords": "todo"
}
```

**Listing all project memories** (scope required, project required for project scope):
```json
{
  "scope": "project",
  "project": "myapp"
}
```

**Listing global memories** (scope required, no project allowed):
```json
{
  "scope": "global"
}
```

## Git & Commits

- No git repo initialized yet
- When committing: use [Conventional Commits](https://www.conventionalcommits.org/) format
- Format: `type(scope): description` (e.g., `feat(mcp): add search filter`)
- Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`
- Branch naming: `feature/`, `fix/`, `chore/`

## Cursor/Copilot Rules

- No `.cursor/rules/` or `.cursorrules` files exist
- No `.github/copilot-instructions.md` exists

## Linting & Formatting

```bash
# Format code
go fmt ./...

# Vet for suspicious constructs
go vet ./...

# No golangci‑lint configuration present
```

## Feature Notes

- **Multi‑Embedding Memory Indexing**: Each memory generates multiple embeddings (main text, project‑only, first three keywords) for improved search across different dimensions.
- **Batch Search Optimization**: Multi‑vector searches use Qdrant's `SearchBatch` API for reduced network latency and parallel processing. All query embeddings are sent in a single batch request instead of sequential calls.
- **Scope Support**: Memories have three scope types: `global` (accessible from any project/user), `personal` (user‑specific), `project` (project‑specific, default). Backward compatible with existing memories.
- **HTTP API**: Optional HTTP‑only mode for standalone server usage; endpoints at `/api/health`, `/api/who`, `/api/search`, `/api/list`.
- **MCP Integration**: Full MCP server with stdio transport; tools accessible via MCP‑compatible clients.