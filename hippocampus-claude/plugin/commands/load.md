---
description: Load memories for the current project
allowed-tools: ["Bash"]
---

# Hippocampus: Load Memories

This command loads memories from the Hippocampus MCP server and displays them.

## Usage

```
/hippocampus load
```

## What it does

1. Fetches memories for the current project from the Hippocampus API
2. Displays them in a formatted list
3. If no memories exist, suggests creating one

## Example Output

```
🧠 Hippocampus Memories (3 found)

1. my-app — Code Style
   Uses TypeScript with strict mode enabled.
   Keywords: typescript, strict, types

2. my-app — Project Setup
   PostgreSQL with Drizzle ORM for database.
   Keywords: database, postgresql, drizzle
```

## Related Commands

- `/hippocampus save` - Save a new memory
- `/hippocampus search` - Search memories
- `/hippocampus list` - List all memories
