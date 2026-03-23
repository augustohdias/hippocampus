#!/usr/bin/env node
/**
 * Hippocampus Session Stop Hook
 * Detects memory creation patterns and suggests saving
 */

const HTTP_API_BASE = process.env.HIPPOCAMPUS_HTTP_URL || "http://localhost:8765/api";

// Memory creation patterns - synchronized with OpenCode plugin
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

function writeOutput(data) {
  console.log(JSON.stringify(data));
}

async function readInput() {
  return new Promise((resolve) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => { data += chunk; });
    process.stdin.on('end', () => {
      try {
        resolve(data.trim() ? JSON.parse(data) : {});
      } catch {
        resolve({});
      }
    });
  });
}

async function readTranscript(transcriptPath) {
  try {
    const fs = await import('fs');
    const content = fs.readFileSync(transcriptPath, 'utf8');
    return content;
  } catch {
    return "";
  }
}

function detectMemoryPatterns(transcript) {
  const matches = [];
  
  for (const pattern of MEMORY_PATTERNS) {
    const match = transcript.match(pattern);
    if (match) {
      matches.push(match[0]);
    }
  }
  
  return matches;
}

async function main() {
  const input = await readInput();
  const transcriptPath = input.transcript_path;
  
  let additionalContext = "";
  
  if (transcriptPath) {
    const transcript = await readTranscript(transcriptPath);
    const patterns = detectMemoryPatterns(transcript);
    
    if (patterns.length > 0) {
      additionalContext = `\n\n<hippocampus_nudge>\n🧠 **Memory Creation Detected**\n\n`;
      additionalContext += `This conversation contains patterns suggesting you want to remember something:\n`;
      additionalContext += `- Found: "${patterns.join('", "')}"\n\n`;
      additionalContext += `**Guidelines for saving memories:**\n`;
      additionalContext += `- Extract key information and save as concise, searchable memories\n`;
      additionalContext += `- If investigating, save only conclusions about what needs investigation\n`;
      additionalContext += `- When examining code, store info about core components (business rules, execution flow)\n`;
      additionalContext += `- Prefer multiple small memories instead of one big memory\n\n`;
      additionalContext += `**To save this as a memory, use the \`hippocampus\` MCP tool:**\n`;
      additionalContext += `\`\`\`json\n{\n  "project": "${input.cwd ? input.cwd.split('/').pop() : 'current-project'}",\n  "content": "Key information to remember",\n  "context": "Category (e.g., Code Patterns, Project Setup)",\n  "keywords": "keyword1, keyword2"\n}\n\`\`\`\n\n`;
      additionalContext += `*The user explicitly asked you to remember. Do not skip this step.*\n`;
      additionalContext += `</hippocampus_nudge>`;
    }
  }

  writeOutput({
    continue: true,
    additionalContext: additionalContext || undefined,
  });
}

main().catch(err => {
  writeOutput({ continue: true });
});
