#!/usr/bin/env node
/**
 * Hippocampus Session Start Hook
 * Loads memories and injects them into the session context
 */

const HIPPOCAMPUS_BINARY = process.env.HIPPOCAMPUS_BINARY || "hippocampus";
const DEFAULT_PORT = 8765;
const PORT_RANGE = 10; // Try ports 8765-8774

function writeOutput(data) {
  console.log(JSON.stringify(data));
}

// Debug logging to file (when HIPPOCAMPUS_DEBUG=true)
const LOG_FILE = "/tmp/hippocampus-cc-plugin.log";
function log(message, data) {
  if (process.env.HIPPOCAMPUS_DEBUG === "true") {
    try {
      const fs = require('fs');
      const timestamp = new Date().toISOString();
      const dataStr = data ? ` ${JSON.stringify(data, null, 2)}` : "";
      const logLine = `[${timestamp}] [hippocampus-cc-plugin] ${message}${dataStr}\n`;
      fs.appendFileSync(LOG_FILE, logLine, { encoding: 'utf8' });
    } catch (error) {
      // If file writing fails, fallback to silent (no output)
    }
  }
}

async function readInput() {
  return new Promise((resolve) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => { data += chunk; });
    process.stdin.on('end', () => {
      try {
        resolve(data.trim() ? JSON.parse(data) : { cwd: process.cwd() });
      } catch {
        resolve({ cwd: process.cwd() });
      }
    });
  });
}

function getProjectName(cwd) {
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
  const available = apiBase !== null;
  log("Checking API availability", { available, apiBase });
  return available;
}

async function startHippocampusServer() {
  try {
    const { spawn } = require('child_process');
    log("Starting hippocampus server with setsid", { binary: HIPPOCAMPUS_BINARY });
    
    // Spawn server with setsid to detach from terminal
    const proc = spawn('setsid', [HIPPOCAMPUS_BINARY, '--http'], {
      stdio: 'ignore',
      detached: true,
    });
    proc.unref();
    
    log("Server spawned with setsid", { pid: proc.pid });
    
    // Wait a bit for server to start
    await new Promise(resolve => setTimeout(resolve, 2000));
    
    // Check if process is still alive (optional)
    const alive = !proc.killed;
    log("Server status after 2s", { pid: proc.pid, alive });
    return alive;
  } catch (error) {
    log("Failed to spawn with setsid", { error: error.message });
    
    // Fallback to direct spawn
    try {
      const { spawn } = require('child_process');
      log("Trying fallback spawn without setsid", { binary: HIPPOCAMPUS_BINARY });
      
      const proc = spawn(HIPPOCAMPUS_BINARY, ['--http'], {
        stdio: 'ignore',
        detached: true,
      });
      proc.unref();
      
      log("Server spawned without setsid", { pid: proc.pid });
      await new Promise(resolve => setTimeout(resolve, 2000));
      
      const alive = !proc.killed;
      log("Server status after 2s (fallback)", { pid: proc.pid, alive });
      return alive;
    } catch (error2) {
      log("Fallback spawn also failed", { error: error2.message });
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

async function fetchMemories(project) {
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
    // Try project-specific search first
    const searchUrl = `${apiBase}/search?project=${encodeURIComponent(project)}&limit=10`;
    const searchResp = await fetch(searchUrl);
    
    if (searchResp.ok) {
      const searchData = await searchResp.json();
      if (searchData.results && searchData.results.length > 0) {
        return searchData.results.map(r => r.memory);
      }
    }

    // Fallback to list all (conservative token usage: limit 10)
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

function formatMemoriesContext(memories) {
  if (!memories || memories.length === 0) {
    return "";
  }

  let context = `<hippocampus_memories>\n`;
  context += `**${memories.length} memories loaded for this session**\n\n`;
  
  memories.forEach((m, idx) => {
    context += `**Memory ${idx + 1}: ${m.project}** (${m.context})\n`;
    context += `${m.content}\n`;
    context += `_Keywords: ${m.keywords.join(", ")}_\n\n`;
  });

  context += `</hippocampus_memories>`;
  return context;
}

async function main() {
  const input = await readInput();
  const cwd = input.cwd || process.cwd();
  const project = getProjectName(cwd);

  const memories = await fetchMemories(project);
  
  let additionalContext = "";
  
  if (memories === null) {
    additionalContext = `<hippocampus_memories>\n⚠️ Could not connect to Hippocampus API. Make sure the MCP server is running.\n</hippocampus_memories>`;
  } else if (memories.length > 0) {
    additionalContext = formatMemoriesContext(memories);
  } else {
    additionalContext = `<hippocampus_memories>\n📭 No memories found for project "${project}".\n</hippocampus_memories>`;
  }

  writeOutput({
    hookSpecificOutput: {
      hookEventName: "SessionStart",
      additionalContext,
    },
  });
}

main().catch(err => {
  writeOutput({
    hookSpecificOutput: {
      hookEventName: "SessionStart",
      additionalContext: `<hippocampus_memories>\n❌ Error: ${err.message}\n</hippocampus_memories>`,
    },
  });
});
