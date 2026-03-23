import type { Plugin, PluginInput } from "@opencode-ai/plugin";
import type { Part } from "@opencode-ai/sdk";
import { log } from "./services/logger";

const PLUGIN_NAME = "hippocampus";

// Track session state for memory re-injection after context compaction
interface SessionState {
  hasInjectedMemories: boolean;
  lastCompactionAt?: number;
}

const sessionStates = new Map<string, SessionState>();

// Memory creation patterns (Portuguese)
const MEMORY_PATTERNS = [
  /\blembre-se\b/i,
  /\bse lembre\b/i,
  /\bmemorize\b/i,
  /\bguarde\b/i,
  /\bguarde em memória\b/i,
  /\bguarde em memoria\b/i,
  /\bcrie uma memória\b/i,
  /\bcrie memória\b/i,
  /\bcrie memorias\b/i,
  /\bcrie uma memoria\b/i,
  /\bcrie memorías\b/i,
];

const MEMORY_NUDGE_MESSAGE = `[HIPPOCAMPUS MEMORY CREATION TRIGGER]

The user wants you to create a memory. You MUST use the \`hippocampus_create_memory\` tool. Try to split the information in multiple small memories to prevent context flooding:

**Required fields:**
- \`content\`: The information to remember (max 250 words)
- \`context\`: Category (e.g., "Code Style", "Project Setup", "Preferences")
- \`keywords\`: Comma-separated keywords for search

**Optional fields:**
- \`scope\`: Memory scope (optional, default: "project")

**Special fields:**
- \`project\`: Project name. This is mandatory if the scope is also "project".

**Scope Guidance:**
1. **Default scope is "project"**: Use for project-specific information (code style, setup, conventions).
2. **Global scope ("global")**: Must be EXPLICITLY requested by the user. Use for information that applies across all projects (e.g., general programming knowledge, cross-project patterns).
3. **Personal scope ("personal")**: Use for user-specific personal information that you should remember about the user: name, age, hobbies, conversation tone preferences, personal preferences. You should decide how to manage this information respectfully.

**Example (project scope - default):**
{
  "project": "my-app",
  "content": "Uses TypeScript with strict mode. All functions must have explicit return types.",
  "context": "Code Style",
  "keywords": "typescript, strict, types"
  "scope": "project"
}

**Example (global scope):**
{
  "project": "my-app",  // Project field is still required but may be ignored for embedding
  "content": "Always use semantic versioning for all projects.",
  "context": "General Best Practices",
  "keywords": "versioning, semver, releases",
  "scope": "global"
}

**Example (personal scope):**
{
  "project": "personal",
  "content": "User prefers casual conversation tone with emojis.",
  "context": "Conversation Preferences",
  "keywords": "tone, preferences, personal",
  "scope": "personal"
}

DO NOT skip this step. The user explicitly asked you to remember this information.`;

const HIPPOCAMPUS_BINARY = process.env.HIPPOCAMPUS_BINARY || "hippocampus";
const DEFAULT_PORT = 8765;
const PORT_RANGE = 10; // Try ports 8765-8774

// Helper to find hippocampus binary in common locations
function getHippocampusBinary(): string {
  // First try environment variable
  if (process.env.HIPPOCAMPUS_BINARY) {
    return process.env.HIPPOCAMPUS_BINARY;
  }

  // Common installation paths
  const possiblePaths = [
    "hippocampus", // In PATH
    "/usr/local/bin/hippocampus",
    "/usr/bin/hippocampus",
    `${process.env.HOME}/.local/bin/hippocampus`,
  ];

  // Return the first path (will be validated at spawn time)
  return possiblePaths[0];
}

// Helper function to check if a server at a given base URL is hippocampus
async function isHippocampusServer(baseUrl: string): Promise<boolean> {
  try {
    log(`Checking if ${baseUrl} is hippocampus server`);

    // First try the /api/who endpoint (more specific)
    const whoUrl = `${baseUrl}/who`;
    log(`Trying ${whoUrl}`);
    const whoResponse = await fetch(whoUrl, {
      signal: AbortSignal.timeout(1000),
    });
    if (whoResponse.ok) {
      const whoData = await whoResponse.json();
      const isHippo = whoData.service === "hippocampus";
      log(`/api/who response`, {
        status: whoResponse.status,
        service: whoData.service,
        isHippo,
      });
      return isHippo;
    }
    log(`/api/who failed with status ${whoResponse.status}`);

    // Fallback to /api/health endpoint (for backward compatibility)
    const healthUrl = `${baseUrl}/health`;
    log(`Trying ${healthUrl}`);
    const healthResponse = await fetch(healthUrl, {
      signal: AbortSignal.timeout(1000),
    });
    if (healthResponse.ok) {
      const healthData = await healthResponse.json();
      const isHippo = healthData.service === "hippocampus";
      log(`/api/health response`, {
        status: healthResponse.status,
        service: healthData.service,
        isHippo,
      });
      return isHippo;
    }
    log(`/api/health failed with status ${healthResponse.status}`);

    log(`Neither endpoint succeeded for ${baseUrl}`);
    return false;
  } catch (error) {
    log(`Error checking if ${baseUrl} is hippocampus server`, {
      error: String(error),
    });
    return false;
  }
}

// Helper functions for server management
async function discoverHippocampusPort(): Promise<string | null> {
  // If environment variable is set, use it directly
  const envUrl = process.env.HIPPOCAMPUS_HTTP_URL;
  if (envUrl) {
    log("Using HIPPOCAMPUS_HTTP_URL from env", { envUrl });
    // Extract port from URL if it's a full URL, or use as-is
    try {
      const url = new URL(envUrl);
      return envUrl; // Return full URL
    } catch {
      // Assume it's already a full URL or just a port
      // If it's just a number, assume it's a port
      if (/^\d+$/.test(envUrl)) {
        return `http://localhost:${envUrl}/api`;
      }
      return envUrl;
    }
  }

  log(
    `Scanning ports ${DEFAULT_PORT} to ${DEFAULT_PORT + PORT_RANGE - 1} for hippocampus server`,
  );

  // Try ports in sequence
  for (let i = 0; i < PORT_RANGE; i++) {
    const port = DEFAULT_PORT + i;
    const baseUrl = `http://localhost:${port}/api`;
    log(`Checking port ${port}...`);
    try {
      // Check if this is a hippocampus server
      if (await isHippocampusServer(baseUrl)) {
        log(`Found hippocampus server on port ${port}`);
        return baseUrl;
      }
      log(`Port ${port} is not hippocampus server`);
    } catch (error) {
      log(`Error checking port ${port}`, { error: String(error) });
      // Continue to next port
    }
  }

  log("No hippocampus server found on any port");
  return null;
}

async function getApiBase(): Promise<string | null> {
  return await discoverHippocampusPort();
}

async function checkApiAvailable(): Promise<boolean> {
  const apiBase = await getApiBase();
  const available = apiBase !== null;
  log("Checking API availability", { available, apiBase });
  return available;
}

async function startHippocampusServer(): Promise<boolean> {
  try {
    const binary = getHippocampusBinary();
    log("Starting hippocampus server with setsid", { binary });

    // Spawn server with setsid to detach from terminal, in background with HTTP-only mode
    // Using setsid to create new session and avoid SIGHUP
    const proc = Bun.spawn(["setsid", binary, "--http"], {
      stdin: "ignore",
      stdout: "ignore",
      stderr: "ignore",
      detached: true,
    });

    log("Server spawned with setsid", { pid: proc.pid });

    // Wait a bit for server to start
    await new Promise((resolve) => setTimeout(resolve, 2000));

    const alive = !proc.killed;
    log("Server status after 2s", { pid: proc.pid, alive });

    // Check if process is still alive (optional)
    return alive;
  } catch (error) {
    log("Failed to spawn with setsid", { error: String(error) });

    // Fallback to direct spawn without setsid
    try {
      const binary = getHippocampusBinary();
      log("Trying fallback spawn without setsid", { binary });

      const proc = Bun.spawn([binary, "--http"], {
        stdin: "ignore",
        stdout: "ignore",
        stderr: "ignore",
        detached: true,
      });

      log("Server spawned without setsid", { pid: proc.pid });
      await new Promise((resolve) => setTimeout(resolve, 2000));

      const alive = !proc.killed;
      log("Server status after 2s (fallback)", { pid: proc.pid, alive });
      return alive;
    } catch (error2) {
      log("Fallback spawn also failed", { error: String(error2) });
      return false;
    }
  }
}

async function ensureApiAvailable(): Promise<boolean> {
  log("Ensuring API is available");

  // First check if API is already available
  if (await checkApiAvailable()) {
    log("API already available");
    return true;
  }

  log("API not available, attempting to start server");

  // Try to start the server
  const started = await startHippocampusServer();
  if (!started) {
    log("Failed to start server");
    return false;
  }

  log("Server started, giving time to initialize...");

  // Give server more time to start (connect to Qdrant, load Ollama model, etc.)
  await new Promise((resolve) => setTimeout(resolve, 5000));

  // Wait and retry check with more attempts
  for (let i = 0; i < 10; i++) {
    log(`Checking API availability attempt ${i + 1}/10`);
    if (await checkApiAvailable()) {
      log(`API available after ${i + 1} attempt(s)`);
      return true;
    }
    log(`API not available yet, waiting 2s...`);
    await new Promise((resolve) => setTimeout(resolve, 2000));
  }

  log("API still not available after all attempts");
  return false;
}

interface Memory {
  id: number;
  project: string;
  context: string;
  content: string;
  keywords: string[];
  scope?: string; // Optional for backward compatibility
}

interface SearchResponse {
  results: Array<{
    memory: Memory;
    score: number;
  }>;
  total: number;
}

interface ListResponse {
  memories: Memory[];
  total: number;
}

async function fetchMemoriesHTTP(project: string): Promise<Memory[] | null> {
  log("fetchMemoriesHTTP called", { project });

  // Ensure API is available (start server if needed)
  const apiAvailable = await ensureApiAvailable();
  if (!apiAvailable) {
    log("API not available, cannot fetch memories");
    return null;
  }

  // Get the actual API base URL (with discovered port)
  const apiBase = await getApiBase();
  if (!apiBase) {
    log("API base URL not found");
    return null;
  }

  log("Attempting to fetch memories with scope logic", { project, apiBase });

  try {
    // Step 1: Fetch ALL project-specific memories with scope=project
    const projectListUrl = `${apiBase}/list?project=${encodeURIComponent(project)}&scope=project&limit=1000`;
    log("Fetching ALL project-specific memories with scope=project", {
      projectListUrl,
    });
    let memories: Memory[] = [];

    const projectListResp = await fetch(projectListUrl);
    if (projectListResp.ok) {
      const projectListData: ListResponse = await projectListResp.json();
      log("Project list response", {
        memoriesCount: projectListData.memories?.length || 0,
      });
      if (projectListData.memories && projectListData.memories.length > 0) {
        const projectMemories = projectListData.memories.map((m) => ({
          ...m,
          scope: m.scope || "project",
        }));
        memories = [...memories, ...projectMemories];
        log(`Added ${projectMemories.length} project memories`);
      } else {
        log("No project-specific memories found");
      }
    } else {
      log("Project list request failed", { status: projectListResp.status });
    }

    // Step 2: Fetch ALL personal memories (scope=personal) with high limit
    const personalListUrl = `${apiBase}/list?scope=personal&limit=1000`;
    log("Fetching ALL personal memories", { personalListUrl });
    const personalListResp = await fetch(personalListUrl);

    if (personalListResp.ok) {
      const personalListData: ListResponse = await personalListResp.json();
      log("Personal list response", {
        memoriesCount: personalListData.memories?.length || 0,
      });
      if (personalListData.memories && personalListData.memories.length > 0) {
        const personalMemories = personalListData.memories.map((m) => ({
          ...m,
          scope: "personal",
        }));
        memories = [...memories, ...personalMemories];
        log(`Added ${personalMemories.length} personal memories`);
      } else {
        log("No personal memories found");
      }
    } else {
      log("Personal list request failed", { status: personalListResp.status });
    }

    // Step 3: Fetch LAST 50 global memories (scope=global)
    const globalListUrl = `${apiBase}/list?scope=global&limit=50`;
    log("Fetching LAST 50 global memories", { globalListUrl });
    const globalListResp = await fetch(globalListUrl);

    if (globalListResp.ok) {
      const globalListData: ListResponse = await globalListResp.json();
      log("Global list response", {
        memoriesCount: globalListData.memories?.length || 0,
      });
      if (globalListData.memories && globalListData.memories.length > 0) {
        const globalMemories = globalListData.memories.map((m) => ({
          ...m,
          scope: "global",
        }));
        memories = [...memories, ...globalMemories];
        log(`Added ${globalMemories.length} global memories`);
      } else {
        log("No global memories found");
      }
    } else {
      log("Global list request failed", { status: globalListResp.status });
    }

    log(`Total memories found: ${memories.length}`);
    return memories;
  } catch (error) {
    // HTTP failed, return null
    log("HTTP error fetching memories", { error: String(error) });
    return null;
  }
}

const MEMORY_INJECTION_HEADER =
  "[HIPPOCAMPUS MEMORIES]\nThe following memories are related to the current context.";

function formatMemoriesContext(memories: Memory[]): string {
  if (memories.length === 0) {
    return "";
  }

  const header = MEMORY_INJECTION_HEADER;
  const body = memories
    .map((m) => {
      const scope = m.scope ? `Scope: ${m.scope}` : "";
      return `Project: ${m.project}
Context: ${m.context}
${scope ? scope + "\n" : ""}Content: ${m.content}
Keywords: ${m.keywords.join(", ")}`;
    })
    .join("\n---\n");

  return `${header}

${body}`;
}

export const HippocampusPlugin: Plugin = async (ctx: PluginInput) => {
  log("HippocampusPlugin initialized", { directory: ctx.directory });

  const { directory } = ctx;
  const pathParts = directory.split("/").filter((p) => p && p !== "opencode");
  const project =
    pathParts.length > 0 ? pathParts[pathParts.length - 1] : "default";

  log("Determined project name", { project, pathParts });

  return {
    "chat.message": async (input, output) => {
      log("chat.message handler called", {
        sessionID: input.sessionID,
        project,
      });

      const sessionKey = input.sessionID;

      // Get or create session state
      let state = sessionStates.get(sessionKey);
      if (!state) {
        state = { hasInjectedMemories: false };
        sessionStates.set(sessionKey, state);
        log("Created new session state", { sessionKey });
      } else {
        log("Existing session state", {
          sessionKey,
          hasInjectedMemories: state.hasInjectedMemories,
        });
      }

      // Re-inject memories if:
      // - First time in session, OR
      // - Compaction was detected (hasInjectedMemories = false)
      if (!state.hasInjectedMemories) {
        log("Injecting memories for first time or after compaction");

        // Try HTTP API first
        const memories = await fetchMemoriesHTTP(project);

        if (memories === null) {
          log("Failed to fetch memories (HTTP API unavailable or error)");
        } else if (memories.length === 0) {
          log("No memories found for this project");
        } else {
          log(`Fetched ${memories.length} memories, injecting into context`);

          const contextText = formatMemoriesContext(memories);

          if (!output.parts || !Array.isArray(output.parts)) {
            output.parts = [];
          }

          output.parts.unshift({
            id: `prt:${PLUGIN_NAME}:ctx:${Date.now()}`,
            type: "text" as const,
            text: contextText,
            sessionID: input.sessionID as string,
            messageID: output.message.id as string,
            synthetic: true,
          });

          state.hasInjectedMemories = true;
          log("Memories injected successfully");
        }
      }

      // Check for memory creation patterns in user message
      log("Checking for memory creation patterns in user message");
      const textParts = output.parts?.filter(
        (p): p is Part & { type: "text"; text: string } => p.type === "text",
      );
      if (textParts && textParts.length > 0) {
        const userMessage = textParts.map((p) => p.text).join("\n");
        log("User message extracted", { length: userMessage.length });

        const detected = MEMORY_PATTERNS.some((pattern) =>
          pattern.test(userMessage),
        );
        log("Memory creation pattern detection", { detected });

        if (detected) {
          log("Memory creation pattern detected, adding nudge message");
          if (!output.parts || !Array.isArray(output.parts)) {
            output.parts = [];
          }

          output.parts.push({
            id: `prt:${PLUGIN_NAME}:nudge:${Date.now()}`,
            type: "text" as const,
            text: MEMORY_NUDGE_MESSAGE,
            sessionID: input.sessionID as string,
            messageID: output.message.id as string,
            synthetic: true,
          });
          log("Nudge message added");
        }
      } else {
        log("No text parts found in output");
      }
    },

    // Monitor OpenCode events to detect context compaction
    event: async (input: { event: { type: string; properties?: unknown } }) => {
      log("Event received", { type: input.event.type });

      if (input.event.type !== "message.updated") {
        log(`Ignoring event type ${input.event.type}`);
        return;
      }

      log("Processing message.updated event");
      const props = input.event.properties as
        | Record<string, unknown>
        | undefined;
      const info = props?.info as
        | {
            id: string;
            role: string;
            sessionID?: string;
            summary?: boolean;
            finish?: boolean;
          }
        | undefined;

      // Detect compaction: assistant message with summary=true and finish=true
      if (
        info?.role === "assistant" &&
        info.summary === true &&
        info.finish === true
      ) {
        log("Context compaction detected", {
          sessionID: info.sessionID,
          role: info.role,
        });
        const sessionID = info.sessionID || (props?.sessionID as string);

        if (sessionID) {
          log(`Looking up session state for ${sessionID}`);
          const state = sessionStates.get(sessionID);
          if (state) {
            // Flag for re-injection on next message
            state.hasInjectedMemories = false;
            state.lastCompactionAt = Date.now();
            log("Session state flagged for re-injection after compaction", {
              sessionID,
            });
          } else {
            log(`No session state found for ${sessionID}`);
          }
        } else {
          log("No sessionID found in event");
        }
      } else {
        log("Not a compaction event", {
          role: info?.role,
          summary: info?.summary,
          finish: info?.finish,
        });
      }
    },
  };
};
