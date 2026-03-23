/**
 * Regex patterns that detect when a user wants to save information.
 * Organized by detection type: explicit commands, implicit preferences, and knowledge statements.
 * Synchronized with Claude Code plugin (hippocampus-claude/src/session-stop.js)
 */
const MEMORY_PATTERNS = [
  // Portuguese patterns
  /\blembre-se\b/i,
  /\bse lembre\b/i,
  /\bmemorize\b/i,
  /\bguarde\b/i,
  /\bguarde em mem[oó]ria\b/i,
  /\bcrie uma mem[oó]ria\b/i,
  /\bcrie mem[oó]rias\b/i,
  /\bcorrija\b/i,
  /\banalise\b/i,

  // English - Explicit save commands
  /\b(remember|save|store|memorize|analyze|investigate|note|record)\b/i,
  /\b(don't forget|do not forget|make sure to remember)\b/i,
  /\b(this is important|keep in mind|keep this in mind)\b/i,
  /\bremember this\b/i,
  /\bsave this to memory\b/i,
  /\bmake a note\b/i,

  // English - Implicit save commands (preferences and conventions)
  /\b(always use|never use|prefer|preferred)\b/i,
  /\b(by convention|convention|standard practice)\b/i,
  /\b(project uses|we use|our setup|our stack)\b/i,
  /\b(configured to|configured for|setup to)\b/i,

  // English - Knowledge statements (workarounds and key insights)
  /\b(the key is|the trick is|important note)\b/i,
  /\b(workaround|solution for|fix for)\b/i,
];

/**
 * Message shown when memory patterns are detected.
 * Instructs the AI to save the information using hippocampus tools.
 */
export const MEMORY_NUDGE_MESSAGE = `[MEMORY TRIGGER DETECTED]
The user wants you to remember something. You MUST use the \`hippocampus\` tool with \`mode: "add"\` to save this information.

Extract the key information the user wants remembered and save it as a concise, searchable memory.
If the request is about investigation, save only the conclusions about what needs to be investigated.
While examining any code base, store information about the core components (business rules, execution flow, anything useful).

- Use \`project\` for the project identifier (current directory name)
- Use \`context\` for categorization (e.g., "Code Patterns", "Project Setup", "Error Solutions")
- Use \`content\` for the actual information to remember (max 250 words)
- Use \`keywords\` for comma-separated keywords to help find the memory later

P.S.: Prefer to store multiple small memories instead of one big memory.

Example:
{
  "mode": "add",
  "project": "my-app",
  "content": "Uses Bun as runtime instead of Node.js. All scripts should use bun run\ndate:2025-12-25",
  "context": "Project Setup",
  "keywords": "bun, runtime, setup, scripts"
}

The user explicitly asked you to remember.`;

/**
 * Result of pattern detection analysis.
 */
export interface PatternMatch {
  detected: boolean;
  pattern?: string;
}

/**
 * Checks if text contains memory-related patterns.
 * @param text - User input text to analyze
 * @returns Match result with detected pattern if found
 */
export function detectMemoryPatterns(text: string): PatternMatch {
  for (const pattern of MEMORY_PATTERNS) {
    if (pattern.test(text)) {
      return {
        detected: true,
        pattern: pattern.toString(),
      };
    }
  }
  return { detected: false };
}
