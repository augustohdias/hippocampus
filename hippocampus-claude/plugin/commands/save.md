---
description: Save a new memory to Hippocampus
allowed-tools: ["Bash", "Read"]
---

# Hippocampus: Save Memory

This command saves a new memory to the Hippocampus MCP server.

## Usage

```
/hippocampus save --content "what to remember" --context "category" [--keywords "kw1,kw2"]
```

## Required Parameters

| Parameter | Description |
|-----------|-------------|
| `--content` | The information to remember (max 250 words) |
| `--context` | Category (e.g., "Code Style", "Project Setup", "Preferences") |

## Optional Parameters

| Parameter | Description |
|-----------|-------------|
| `--project` | Project identifier (default: current directory name) |
| `--keywords` | Comma-separated keywords for search |

## Examples

```bash
# Save a code style preference
/hippocampus save --content "Uses TypeScript with strict mode. All functions must have explicit return types." --context "Code Style" --keywords "typescript,strict,types"

# Save project setup info
/hippocampus save --content "PostgreSQL database with Drizzle ORM. Connection pooling enabled." --context "Project Setup" --keywords "database,postgresql,drizzle"

# Save without keywords
/hippocampus save --content "Team prefers PR reviews within 24 hours" --context "Team Practices"
```

## Memory Creation Patterns

The plugin automatically detects when you want to create a memory. Use phrases like:

- "lembre-se que..."
- "guarde em memória..."
- "memorize que..."
- "remember this..."
- "save this to memory..."

When detected, the plugin will suggest using this command.

## Related Commands

- `/hippocampus load` - Load memories
- `/hippocampus search` - Search memories
- `/hippocampus list` - List all memories
