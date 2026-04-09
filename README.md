![logo](./logo.png)

Locally record and access LLM agent memories across multiple sessions — without flooding your context window with irrelevant information.

**Private, secure, and free.**

Hippocampus provides an MCP server and plugin to manage memory creation, storage, and context injection.

> **Note:** Primarily developed for [opencode](https://opencode.ai). Claude Code hook support is included but not fully tested.

## How it works

Memories are vectorized using any model you choose and stored in a Qdrant collection, enabling semantic search so LLMs can retrieve only what's relevant.

Collections are automatically configured with the correct dimensions on first use.

## Memory Scopes

Hippocampus organizes memories into three scopes:

| Scope        | Behavior                                                                                |
| ------------ | --------------------------------------------------------------------------------------- |
| **global**   | Shared across all projects. Must be explicitly requested.                               |
| **project**  | Default scope. Loaded automatically when you open a session in the same project.        |
| **personal** | Loaded at every session start. Ideal for personal context, separate from workflow data. |

Memories are injected at the start of each session. Each scope loads up to 10 memories by default — configurable via [environment variables](#environment-variables).

## Metadata & Search

Each memory is stored with a **Context/Title** and up to **5 keywords**, all of which are independently vectorized. This multi-embedding approach improves recall — you can ask the agent to search memories at any time using natural language.

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

| Variable                     | Default              | Description                              |
| ---------------------------- | -------------------- | ---------------------------------------- |
| `OLLAMA_MODEL`               | `qwen3-embedding:4b` | Embedding model                          |
| `QDRANT_HOST`                | `localhost:6334`     | Qdrant gRPC endpoint                     |
| `QDRANT_COLLECTION`          | `memories`           | Collection name                          |
| `LOG_LEVEL`                  | `info`               | Logging level                            |
| `HIPPOCAMPUS_HTTP_PORT`      | `8765`               | HTTP API port                            |
| `HIPPOCAMPUS_GLOBAL_LIMIT`   | `10`                 | Max global memories loaded per session   |
| `HIPPOCAMPUS_PROJECT_LIMIT`  | `10`                 | Max project memories loaded per session  |
| `HIPPOCAMPUS_PERSONAL_LIMIT` | `10`                 | Max personal memories loaded per session |

## Uninstall

```bash
make uninstall  # removes binary and plugin (config preserved)
```

## License

MIT
