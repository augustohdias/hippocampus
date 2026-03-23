interface MemoryResult {
  id: number | string;
  project: string;
  context: string;
  content: string;
  keywords: string[];
  similarity?: number | string;
}

/**
 * Formats search results into a readable context block.
 * Used to inject memory context into AI responses.
 * @param memories - Array of memory results
 * @returns Formatted string with memory summaries
 */
export function formatMemoryContext(memories: MemoryResult[]): string {
  if (!memories || memories.length === 0) {
    return "";
  }

  const header = "[HIPPOCAMPUS MEMORIES]";
  
  const memoryBlocks = memories.map((memory) => {
    const similarityInfo = memory.similarity 
      ? typeof memory.similarity === "number" 
        ? `[${(memory.similarity * 100).toFixed(0)}%]`
        : `[${memory.similarity}]`
      : "";

    return `${similarityInfo} Project: ${memory.project}
Context: ${memory.context}
Content: ${memory.content}
Keywords: ${memory.keywords.join(", ")}
---`;
  });

  return `${header}

Relevant memories found:

${memoryBlocks.join("\n")}

Use these memories to inform your responses and maintain consistency with previously stored knowledge.`;
}
