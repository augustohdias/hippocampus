# Hippocampus MCP Server

An MCP (Model Context Protocol) server for creating and searching memories using embeddings and vector database.

## Prerequisites

- Go 1.25+
- Docker and Docker Compose (for Qdrant)
- Ollama installed locally

## Quick Install

```bash
make install      # binary + plugin + OpenCode config + start Qdrant/Ollama
make install-yes  # same with auto-confirm (CI/CD)
```

## Manual Install

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd hippocampus
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Configure environment (optional):
   ```bash
   cp .env.example .env
   ```

4. Start Qdrant:
   ```bash
   docker compose up -d qdrant
   ```

5. Pull the embedding model and start Ollama:
   ```bash
   ollama pull qwen3-embedding:4b
   ollama serve
   ```

6. Build and run:
   ```bash
   make build
   make run
   ```

## Configure in Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "hippocampus": {
      "command": "/absolute/path/to/bin/hippocampus",
      "args": []
    }
  }
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_MODEL` | `qwen3-embedding:4b` | Embedding model |
| `QDRANT_HOST` | `localhost:6334` | Qdrant gRPC endpoint |
| `QDRANT_COLLECTION` | `memories` | Collection name |
| `LOG_LEVEL` | `info` | Logging level |
| `HIPPOCAMPUS_HTTP_PORT` | `8765` | HTTP API port |
| `HIPPOCAMPUS_HTTP_ONLY` | (none) | Set to `"true"` to run HTTP-only mode |

## Uninstall

```bash
make uninstall  # removes binary and plugin (config preserved)
```

## License

MIT
