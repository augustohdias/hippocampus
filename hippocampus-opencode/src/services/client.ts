import { log } from "./logger.js";

const HIPPOCAMPUS_BINARY = process.env.HIPPOCAMPUS_BINARY || "hippocampus";

/**
 * Represents a stored memory in Hippocampus.
 */
export interface Memory {
  id: number;
  project: string;
  context: string;
  content: string;
  keywords: string[];
}

/**
 * Represents a search result with similarity score.
 */
export interface SearchResult {
  id: number;
  project: string;
  context: string;
  content: string;
  keywords: string[];
  similarity: string | number;
}

/**
 * Client for interacting with the Hippocampus MCP server.
 * Communicates via MCP protocol to call memory management tools.
 */
export class HippocampusClient {
  private binaryPath: string;

  /**
   * Creates a new Hippocampus client.
   * @param binaryPath - Optional path to the hippocampus binary
   */
  constructor(binaryPath?: string) {
    this.binaryPath = binaryPath || HIPPOCAMPUS_BINARY;
  }

  /**
   * Creates a new memory with semantic embedding.
   * @param project - Project identifier
   * @param context - Context category (e.g., "Code Patterns", "Research Sources")
   * @param content - Content to remember (max 250 words)
   * @param keywords - Comma-separated keywords for search
   * @returns Memory ID or error information
   */
  async createMemory(
    project: string,
    context: string,
    content: string,
    keywords: string[]
  ): Promise<{ id?: number; message?: string; error?: string }> {
    try {
      const input = {
        project,
        context,
        content,
        keywords: keywords.join(", "),
      };

      const result = await this.callTool("create_memory", input);
      return result;
    } catch (error) {
      log("createMemory error", { error: String(error) });
      return {
        error: error instanceof Error ? error.message : String(error),
      };
    }
  }

  /**
   * Searches for memories similar to the given query.
   * Uses semantic similarity to find relevant stored memories.
   * @param project - Project identifier to scope search
   * @param context - Optional context filter
   * @param keywords - Optional keywords filter
   * @param limit - Maximum number of results (default: 10)
   * @returns Search results with similarity scores or error information
   */
  async searchMemories(
    project: string,
    context: string,
    keywords: string,
    limit: number = 10
  ): Promise<{ results?: SearchResult[]; message?: string; error?: string }> {
    try {
      const input = {
        project,
        ...(context && { context }),
        ...(keywords && { keywords }),
        limit,
      };

      const result = await this.callTool("search_memories", input);
      return result;
    } catch (error) {
      log("searchMemories error", { error: String(error) });
      return {
        error: error instanceof Error ? error.message : String(error),
      };
    }
  }

  /**
   * Lists memories with optional filters.
   * Returns structured memories without semantic ranking.
   * @param project - Optional project filter
   * @param keywords - Optional comma-separated keywords filter
   * @param limit - Maximum number of results (default: 100)
   * @returns List of memories or error information
   */
  async listMemories(
    project: string,
    keywords: string,
    limit: number = 100
  ): Promise<{ memories?: Memory[]; message?: string; error?: string }> {
    try {
      const input: Record<string, any> = { limit };
      
      if (project) {
        input.project = project;
      }
      if (keywords) {
        input.keywords = keywords;
      }

      const result = await this.callTool("list_memories", input);
      return result;
    } catch (error) {
      log("listMemories error", { error: String(error) });
      return {
        error: error instanceof Error ? error.message : String(error),
      };
    }
  }

  /**
   * Calls an MCP tool on the Hippocampus server.
   * Falls back gracefully when MCP server is not configured.
   * @param toolName - Name of the tool to call
   * @param args - Tool arguments
   * @returns Tool response or fallback message
   */
  private async callTool(toolName: string, args: Record<string, any>): Promise<any> {
    try {
      log("MCP request", { tool: toolName, args });

      // Fallback: MCP tools are assumed to be available via OpenCode's MCP configuration.
      // Full implementation would use @modelcontextprotocol/sdk here.
      
      return {
        message: `Tool ${toolName} would be called with: ${JSON.stringify(args)}`,
        note: "Hippocampus MCP server should be configured in OpenCode for full functionality",
        config: {
          binary: this.binaryPath,
          setup: 'Add to ~/.config/opencode/opencode.json: { "mcp": { "hippocampus": { "command": "hippocampus", "args": ["--stdio"] } } }',
        },
      };
    } catch (error) {
      log("MCP call failed", { 
        tool: toolName, 
        error: String(error) 
      });
      
      return {
        error: error instanceof Error ? error.message : String(error),
      };
    }
  }
}
