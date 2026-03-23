#!/usr/bin/env node
/**
 * Hippocampus Command Handler
 * Handles all /hippocampus:* commands
 */

const HIPPOCAMPUS_BINARY = process.env.HIPPOCAMPUS_BINARY || "hippocampus";
const DEFAULT_PORT = 8765;
const PORT_RANGE = 10; // Try ports 8765-8774

function getProjectName() {
  const cwd = process.cwd();
  const pathParts = cwd.split("/").filter(p => p && p !== "opencode");
  return pathParts.length > 0 ? pathParts[pathParts.length - 1] : "default";
}

// Helper function to check if a server at a given base URL is hippocampus
async function isHippocampusServer(baseUrl) {
  try {
    // First try the /api/who endpoint (more specific)
    const whoUrl = `${baseUrl}/who`;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 1000);
    const whoResponse = await fetch(whoUrl, { signal: controller.signal });
    clearTimeout(timeout);
    
    if (whoResponse.ok) {
      const whoData = await whoResponse.json();
      return whoData.service === 'hippocampus';
    }
    
    // Fallback to /api/health endpoint (for backward compatibility)
    const healthUrl = `${baseUrl}/health`;
    const healthController = new AbortController();
    const healthTimeout = setTimeout(() => healthController.abort(), 1000);
    const healthResponse = await fetch(healthUrl, { signal: healthController.signal });
    clearTimeout(healthTimeout);
    
    if (healthResponse.ok) {
      const healthData = await healthResponse.json();
      return healthData.service === 'hippocampus';
    }
    
    return false;
  } catch {
    return false;
  }
}

// Helper functions for server management
async function discoverHippocampusPort() {
  // If environment variable is set, use it directly
  const envUrl = process.env.HIPPOCAMPUS_HTTP_URL;
  if (envUrl) {
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

  // Try ports in sequence
  for (let i = 0; i < PORT_RANGE; i++) {
    const port = DEFAULT_PORT + i;
    const baseUrl = `http://localhost:${port}/api`;
    try {
      // Check if this is a hippocampus server
      if (await isHippocampusServer(baseUrl)) {
        return baseUrl;
      }
    } catch {
      // Continue to next port
    }
  }
  
  return null;
}

async function getApiBase() {
  return await discoverHippocampusPort();
}

async function checkApiAvailable() {
  const apiBase = await getApiBase();
  return apiBase !== null;
}

async function startHippocampusServer() {
  try {
    const { spawn } = require('child_process');
    // Spawn server with setsid to detach from terminal
    const proc = spawn('setsid', [HIPPOCAMPUS_BINARY, '--http'], {
      stdio: 'ignore',
      detached: true,
    });
    proc.unref();
    
    // Wait a bit for server to start
    await new Promise(resolve => setTimeout(resolve, 2000));
    
    // Check if process is still alive (optional)
    const alive = !proc.killed;
    return alive;
  } catch {
    // Fallback to direct spawn
    try {
      const { spawn } = require('child_process');
      const proc = spawn(HIPPOCAMPUS_BINARY, ['--http'], {
        stdio: 'ignore',
        detached: true,
      });
      proc.unref();
      await new Promise(resolve => setTimeout(resolve, 2000));
      const alive = !proc.killed;
      return alive;
    } catch {
      return false;
    }
  }
}

async function ensureApiAvailable() {
  // First check if API is already available
  if (await checkApiAvailable()) {
    return true;
  }
  
  // Try to start the server
  const started = await startHippocampusServer();
  if (!started) {
    return false;
  }
  
  // Give server more time to start (connect to Qdrant, load Ollama model, etc.)
  await new Promise(resolve => setTimeout(resolve, 5000));
  
  // Wait and retry check with more attempts
  for (let i = 0; i < 10; i++) {
    if (await checkApiAvailable()) {
      return true;
    }
    await new Promise(resolve => setTimeout(resolve, 2000));
  }
  
  return false;
}

async function fetchMemoriesHTTP(project) {
  // Ensure API is available (start server if needed)
  const apiAvailable = await ensureApiAvailable();
  if (!apiAvailable) {
    return null;
  }

  // Get the actual API base URL (with discovered port)
  const apiBase = await getApiBase();
  if (!apiBase) {
    return null;
  }

  try {
    const searchUrl = `${apiBase}/search?project=${encodeURIComponent(project)}&limit=10`;
    const searchResp = await fetch(searchUrl);
    
    if (searchResp.ok) {
      const searchData = await searchResp.json();
      if (searchData.results && searchData.results.length > 0) {
        return searchData.results.map(r => r.memory);
      }
    }

    // Conservative token usage: limit to 10 most recent memories
    const listUrl = `${apiBase}/list?limit=10`;
    const listResp = await fetch(listUrl);
    
    if (listResp.ok) {
      const listData = await listResp.json();
      return listData.memories || [];
    }

    return [];
  } catch {
    return null;
  }
}

function formatMemoriesDisplay(memories) {
  if (memories.length === 0) {
    return "📭 No memories found for this project.";
  }

  let output = `🧠 **Hippocampus Memories** (${memories.length} found)\n\n`;
  
  memories.forEach((m, idx) => {
    output += `**${idx + 1}. ${m.project}** — ${m.context}\n`;
    output += `   ${m.content}\n`;
    output += `   _Keywords: ${m.keywords.join(", ")}_\n\n`;
  });

  return output.trim();
}

async function cmdLoad() {
  const project = getProjectName();
  console.log(`🔍 Loading memories for project: ${project}\n`);
  
  const memories = await fetchMemoriesHTTP(project);
  
  if (memories === null) {
    console.log("⚠️  Could not connect to Hippocampus API.");
    console.log("   Make sure the MCP server is running: `hippocampus`");
    process.exit(1);
  }
  
  console.log(formatMemoriesDisplay(memories));
  
  if (memories.length > 0) {
    console.log("\n💡 These memories are now available in your session context.");
  }
}

async function cmdSave(args) {
  const params = {};
  
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--project" && args[i + 1]) {
      params.project = args[++i];
    } else if (args[i] === "--content" && args[i + 1]) {
      params.content = args[++i];
    } else if (args[i] === "--context" && args[i + 1]) {
      params.context = args[++i];
    } else if (args[i] === "--keywords" && args[i + 1]) {
      params.keywords = args[++i];
    }
  }
  
  if (!params.content) {
    console.log("❌ Missing required parameter: --content");
    console.log("\nUsage:");
    console.log("  /hippocampus save --content <text> --context <ctx> [--project <name>] [--keywords <kws>]");
    console.log("\nExample:");
    console.log('  /hippocampus save --content "Uses TypeScript with strict mode" --context "Code Style" --keywords "typescript,strict"');
    process.exit(1);
  }
  
  console.log("📝 Memory ready to save:\n");
  console.log(`**Project:** ${params.project || getProjectName()}`);
  console.log(`**Context:** ${params.context || "General"}`);
  console.log(`**Content:** ${params.content}`);
  if (params.keywords) {
    console.log(`**Keywords:** ${params.keywords}`);
  }
  console.log("\n✅ To save this memory, the MCP tool `hippocampus_create_memory` will be used.");
  console.log("   The memory will be stored in Qdrant with an embedding for semantic search.");
}

async function cmdSearch(query) {
  if (!query) {
    console.log("❌ Please provide a search query");
    console.log("Usage: /hippocampus search <query>");
    process.exit(1);
  }
  
  // Ensure API is available (start server if needed)
  const apiAvailable = await ensureApiAvailable();
  if (!apiAvailable) {
    console.log("❌ Hippocampus API unavailable and server could not be started");
    process.exit(1);
  }
  
  // Get the actual API base URL (with discovered port)
  const apiBase = await getApiBase();
  if (!apiBase) {
    console.log("❌ Hippocampus API base URL not found");
    process.exit(1);
  }
  
  try {
    const searchUrl = `${apiBase}/search?query=${encodeURIComponent(query)}&limit=10`;
    const searchResp = await fetch(searchUrl);
    
    if (!searchResp.ok) {
      console.log("⚠️  Could not connect to Hippocampus API.");
      process.exit(1);
    }
    
    const searchData = await searchResp.json();
    
    if (searchData.results.length === 0) {
      console.log(`📭 No memories found matching "${query}"`);
      return;
    }
    
    console.log(`🔍 Search results for "${query}" (${searchData.results.length} found):\n`);
    
    searchData.results.forEach((r, idx) => {
      console.log(`**${idx + 1}.** [Score: ${(r.score * 100).toFixed(0)}%] ${r.memory.project} — ${r.memory.context}`);
      console.log(`   ${r.memory.content}`);
      console.log(`   _Keywords: ${r.memory.keywords.join(", ")}_\n`);
    });
  } catch {
    console.log("⚠️  Error searching memories");
    process.exit(1);
  }
}

async function cmdList(project) {
  // Ensure API is available (start server if needed)
  const apiAvailable = await ensureApiAvailable();
  if (!apiAvailable) {
    console.log("❌ Hippocampus API unavailable and server could not be started");
    process.exit(1);
  }
  
  // Get the actual API base URL (with discovered port)
  const apiBase = await getApiBase();
  if (!apiBase) {
    console.log("❌ Hippocampus API base URL not found");
    process.exit(1);
  }
  
  try {
    // Conservative token usage: limit to 10 most recent memories
    let listUrl = `${apiBase}/list?limit=10`;
    if (project) {
      listUrl += `&project=${encodeURIComponent(project)}`;
    }
    
    const listResp = await fetch(listUrl);
    
    if (!listResp.ok) {
      console.log("⚠️  Could not connect to Hippocampus API.");
      process.exit(1);
    }
    
    const listData = await listResp.json();
    
    if (listData.memories.length === 0) {
      console.log("📭 No memories found.");
      return;
    }
    
    console.log(`📚 All memories (${listData.memories.length} total, limited to 10 most recent):\n`);
    
    listData.memories.forEach((m, idx) => {
      console.log(`**${idx + 1}. ${m.project}** — ${m.context}`);
      console.log(`   ${m.content}`);
      console.log(`   _Keywords: ${m.keywords.join(", ")}_\n`);
    });
  } catch {
    console.log("⚠️  Error listing memories");
    process.exit(1);
  }
}

async function main() {
  const args = process.argv.slice(2);
  const command = args[0] || "load";
  const commandArgs = args.slice(1);
  
  switch (command) {
    case "load":
      await cmdLoad();
      break;
    case "add":
      await cmdSave(commandArgs);
      break;
    case "save":
      await cmdSave(commandArgs);
      break;
    case "search":
      await cmdSearch(commandArgs.join(" "));
      break;
    case "list":
      const projectArg = commandArgs.find(a => a.startsWith("--project"));
      const project = projectArg ? (projectArg.split("=")[1] || commandArgs[commandArgs.indexOf("--project") + 1]) : undefined;
      await cmdList(project);
      break;
    case "--help":
    case "-h":
      console.log(`
🧠 Hippocampus — Memory System for Claude Code

Usage: /hippocampus <command> [options]

Commands:
  load              Load memories for current project
  save/add          Save a new memory
  search <query>    Search memories by text
  list              List all memories

Options for save:
  --project <name>   Project identifier (default: current directory)
  --content <text>   Content to remember (required, max 250 words)
  --context <ctx>    Category (default: "General")
  --keywords <kws>   Comma-separated keywords

Examples:
  /hippocampus load
  /hippocampus save --content "Uses TypeScript" --context "Code Style"
  /hippocampus search "typescript"
  /hippocampus list
  /hippocampus list --project my-app

Environment:
  HIPPOCAMPUS_HTTP_URL  API base URL (default: http://localhost:8765/api)
`);
      break;
    default:
      console.log(`❌ Unknown command: ${command}`);
      console.log("Run /hippocampus --help for usage.");
      process.exit(1);
  }
}

main().catch(err => {
  console.error("Error:", err);
  process.exit(1);
});
