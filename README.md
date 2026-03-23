# Hippocampus MCP Server

An MCP (Model Context Protocol) server for creating and searching memories using embeddings and vector database.

## Features

1. **Create memories**: Receives a text block, generates embeddings using Ollama, and stores them in Qdrant.
2. **Search memories**: Receives a query text, searches for similar memories in Qdrant, and returns results sorted by similarity (with percentage).

## Prerequisites

- Go 1.25+
- Docker and Docker Compose (for Qdrant)
- Ollama installed locally
- Embedding model `qwen3-embedding:4b` (will be downloaded automatically)

## Installation

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
   # Edit .env if needed
   ```

## Configuration

### Environment variables

Create a `.env` file based on `.env.example`:

```bash
# Ollama configuration
OLLAMA_MODEL=qwen3-embedding:4b  # Model for embeddings

# Qdrant configuration  
QDRANT_HOST=localhost:6334
QDRANT_COLLECTION=memories

# Server configuration
LOG_LEVEL=info
```

### Start services

1. Start Qdrant:
   ```bash
   make docker-up
   ```

2. Install Ollama model:
   ```bash
   ollama pull qwen3-embedding:4b
   ```

## Usage

### Build and run

```bash
# Build
make build

# Run
make run

# Or use complete setup
make dev
```

### Configure in Claude Desktop

Add to Claude Desktop configuration file (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hippocampus": {
      "command": "/absolute/path/to/hippocampus/bin/hippocampus",
      "args": []
    }
  }
}
```

Or use the full path to the compiled binary.

## Available MCP tools

### `create_memory`
Creates a new memory from text.

**Parameters:**
- `text` (string, required): Text to create memory from

**Example:**
```
create_memory({
  "text": "Go is a compiled, statically typed programming language"
})
```

### `search_memories`
Searches for memories similar to a query text.

**Parameters:**
- `query` (string, required): Text to search for similar memories
- `limit` (number, optional, default: 10): Maximum number of results

**Example:**
```
search_memories({
  "query": "programming language",
  "limit": 5
})
```

**Return:**
```json
{
  "results": [
    {
      "id": 123456789,
      "text": "Go is a compiled, statically typed programming language...",
      "similarity": "85.23%",
      "score": 0.8523
    }
  ],
  "total": 1
}
```

## Architecture

```
┌─────────────┐    ┌──────────┐    ┌──────────┐    ┌────────┐
│   MCP       │───▶│   MCP    │───▶│  Ollama  │───▶│Embedding│
│   Client    │    │  Server  │    │  (CLI)   │    │ (Text)  │
└─────────────┘    └──────────┘    └──────────┘    └────────┘
                        │                 │
                        ▼                 ▼
                   ┌──────────┐    ┌──────────┐
                   │  Qdrant  │◀───│  Vectors │
                   │ (Vector  │    │(Embeddings)│
                   │ Database)│    └──────────┘
                   └──────────┘
```

## Development

### Project structure

```
├── mcp/                 # Go MCP server implementation
│   ├── cmd/hippocampus/ # Entry point
│   └── internal/
│       ├── config/      # Configuration and environment variables
│       ├── ollama/      # Ollama client (via CLI)
│       ├── qdrant/      # Qdrant client
│       └── mcp/         # MCP tool implementations
├── hippocampus-opencode/     # OpenCode plugin (TypeScript)
├── hippocampus-claude/           # Claude Code plugin (JavaScript)
├── setup/               # Installation and utility scripts
├── tasks/               # Development task tracking
├── .env.example         # Example environment variables
├── docker-compose.yml   # Qdrant configuration
└── Makefile             # Useful commands
```

### Tests

```bash
make test
```

### Cleanup

```bash
make clean
make docker-down
```

## License

MIT