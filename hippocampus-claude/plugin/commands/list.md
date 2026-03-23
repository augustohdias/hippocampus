---
description: List all memories
allowed-tools: ["Bash"]
---

# Hippocampus: List Memories

This command lists all your stored memories.

## Usage

```
/hippocampus list [--project <name>]
```

## Options

| Option | Description |
|--------|-------------|
| `--project <name>` | Filter by project name |

## Examples

```bash
# List all memories
/hippocampus list

# List memories for a specific project
/hippocampus list --project my-app
```

## Output

```
📚 All memories (5 total):

1. my-app — Code Style
   Uses TypeScript with strict mode enabled.
   Keywords: typescript, strict, types

2. my-app — Project Setup
   PostgreSQL database with Drizzle ORM.
   Keywords: database, postgresql, drizzle

3. another-project — Dependencies
   React 18 with Next.js 14.
   Keywords: react, nextjs, frontend
```

## Related Commands

- `/hippocampus load` - Load memories
- `/hippocampus save` - Save a new memory
- `/hippocampus search` - Search memories
