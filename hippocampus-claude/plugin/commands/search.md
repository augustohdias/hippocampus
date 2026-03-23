---
description: Search memories by text
allowed-tools: ["Bash"]
---

# Hippocampus: Search Memories

This command searches your memories using semantic search.

## Usage

```
/hippocampus search <query>
```

## Examples

```bash
# Search for TypeScript-related memories
/hippocampus search typescript

# Search for database setup
/hippocampus search postgresql database

# Search with multiple words
/hippocampus search code style preferences
```

## How it works

The search uses semantic similarity (not just keyword matching), so it can find memories even if they don't contain the exact words you search for.

## Output

Results are sorted by relevance score (highest first):

```
🔍 Search results for "typescript" (2 found):

1. [Score: 92%] my-app — Code Style
   Uses TypeScript with strict mode enabled.
   Keywords: typescript, strict, types

2. [Score: 78%] another-project — Dependencies
   TypeScript 5.x with ES2022 target.
   Keywords: typescript, version, es2022
```

## Related Commands

- `/hippocampus load` - Load memories
- `/hippocampus save` - Save a new memory
- `/hippocampus list` - List all memories
