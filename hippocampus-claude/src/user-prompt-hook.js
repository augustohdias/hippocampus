#!/usr/bin/env node
/**
 * Hippocampus UserPromptSubmit Hook
 * Detects memory-creation patterns in the user prompt and injects a nudge
 * instructing Claude to use the hippocampus_create_memory MCP tool.
 */

function writeOutput(additionalContext) {
  const out = { hookSpecificOutput: { hookEventName: "UserPromptSubmit" } };
  if (additionalContext)
    out.hookSpecificOutput.additionalContext = additionalContext;
  console.log(JSON.stringify(out));
}

// Debug logging to file (when HIPPOCAMPUS_DEBUG=true)
const LOG_FILE = "/tmp/hippocampus-cc-plugin.log";
function log(message, data) {
  if (process.env.HIPPOCAMPUS_DEBUG === "true") {
    try {
      const fs = require("fs");
      const timestamp = new Date().toISOString();
      const dataStr = data ? ` ${JSON.stringify(data, null, 2)}` : "";
      fs.appendFileSync(
        LOG_FILE,
        `[${timestamp}] [user-prompt-hook] ${message}${dataStr}\n`,
        { encoding: "utf8" },
      );
    } catch {
      /* silent */
    }
  }
}

async function readInput() {
  return new Promise((resolve) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => {
      data += chunk;
    });
    process.stdin.on("end", () => {
      try {
        resolve(data.trim() ? JSON.parse(data) : {});
      } catch {
        resolve({});
      }
    });
  });
}

// Memory creation patterns (Portuguese) — mirrors index.ts in hippocampus-opencode
const MEMORY_PATTERNS = [
  // Portuguese patterns
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

  // English - Explicit commands
  /\bremember\b/i,
  /\bsave\b/i,
  /\bstore\b/i,
  /\bnote\b/i,
  /\brecord\b/i,
  /\bdon't forget\b/i,
  /\bdo not forget\b/i,
  /\bmake sure to remember\b/i,
  /\bthis is important\b/i,
  /\bkeep in mind\b/i,
  /\bkeep this in mind\b/i,
  /\bremember this\b/i,
  /\bsave this to memory\b/i,
  /\bmake a note\b/i,

  // English - Preferences and conventions
  /\balways use\b/i,
  /\bnever use\b/i,
  /\bprefer\b/i,
  /\bpreferred\b/i,
  /\bby convention\b/i,
  /\bconvention\b/i,
  /\bstandard practice\b/i,
  /\bproject uses\b/i,
  /\bwe use\b/i,
  /\bour setup\b/i,
  /\bour stack\b/i,
  /\bconfigured to\b/i,
  /\bconfigured for\b/i,
  /\bsetup to\b/i,

  // English - Knowledge statements
  /\bthe key is\b/i,
  /\bthe trick is\b/i,
  /\bimportant note\b/i,
  /\bworkaround\b/i,
  /\bsolution for\b/i,
  /\bfix for\b/i,
];

const MEMORY_NUDGE_MESSAGE = `[HIPPOCAMPUS MEMORY CREATION TRIGGER]

The user wants you to create a memory. You MUST use the \`hippocampus_create_memory\` MCP tool. Try to split the information into multiple small memories to prevent context flooding:

**Required fields:**
- \`content\`: The information to remember (max 250 words)
- \`context\`: Category (e.g., "Code Style", "Project Setup", "Preferences")
- \`keywords\`: Comma-separated keywords for search

**Optional fields:**
- \`scope\`: Memory scope (default: "project")

**Special fields:**
- \`project\`: Project name. Mandatory when scope is "project". Use only the last directory name of the current path.

**Scope Guidance:**
1. **Default scope is "project"**: Use for project-specific information (code style, setup, conventions).
2. **Global scope ("global")**: Must be EXPLICITLY requested by the user. Use for information that applies across all projects.
3. **Personal scope ("personal")**: Use for user-specific personal information: name, age, hobbies, conversation tone preferences.

DO NOT skip this step. The user explicitly asked you to remember this information.`;

async function main() {
  const input = await readInput();
  const userPrompt = input.prompt || input.text || "";

  log("UserPromptSubmit hook invoked", { promptLength: userPrompt.length });

  const detected = MEMORY_PATTERNS.some((pattern) => pattern.test(userPrompt));
  log("Memory pattern detection", { detected });

  if (detected) {
    log("Pattern matched — injecting nudge");
    writeOutput(MEMORY_NUDGE_MESSAGE);
  } else {
    writeOutput(undefined);
  }
}

main().catch((err) => {
  log("Unhandled error", { error: err.message });
  writeOutput(undefined);
});
