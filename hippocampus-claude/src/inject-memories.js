#!/usr/bin/env node
/**
 * Hippocampus Memory Injection Hook
 * Fires on: SessionStart, SubagentStart, PostCompact
 * Fetches all memories (project + personal + global) and injects into context.
 */

const HIPPOCAMPUS_BINARY = process.env.HIPPOCAMPUS_BINARY || "hippocampus";
const GLOBAL_MEMORY_LIMIT = parseInt(process.env.HIPPOCAMPUS_GLOBAL_LIMIT) || 10;
const PROJECT_MEMORY_LIMIT = parseInt(process.env.HIPPOCAMPUS_PROJECT_LIMIT) || 10;
const PERSONAL_MEMORY_LIMIT = parseInt(process.env.HIPPOCAMPUS_PERSONAL_LIMIT) || 10;
const DEFAULT_PORT = 8765;
const PORT_RANGE = 10; // Try ports 8765-8774

function writeOutput(eventName, additionalContext) {
  console.log(JSON.stringify({
    hookSpecificOutput: { hookEventName: eventName, additionalContext },
  }));
}

// Debug logging to file (when HIPPOCAMPUS_DEBUG=true)
const LOG_FILE = "/tmp/hippocampus-cc-plugin.log";
function log(message, data) {
  if (process.env.HIPPOCAMPUS_DEBUG === "true") {
    try {
      const fs = require('fs');
      const timestamp = new Date().toISOString();
      const dataStr = data ? ` ${JSON.stringify(data, null, 2)}` : "";
      fs.appendFileSync(LOG_FILE, `[${timestamp}] [inject-memories] ${message}${dataStr}\n`, { encoding: 'utf8' });
    } catch { /* silent */ }
  }
}

async function readInput() {
  return new Promise((resolve) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => { data += chunk; });
    process.stdin.on('end', () => {
      try { resolve(data.trim() ? JSON.parse(data) : { cwd: process.cwd() }); }
      catch { resolve({ cwd: process.cwd() }); }
    });
  });
}

function getProjectName(cwd) {
  const pathParts = (cwd || process.cwd()).split("/").filter(p => p && p !== "opencode");
  return pathParts.length > 0 ? pathParts[pathParts.length - 1] : "default";
}

async function isHippocampusServer(baseUrl) {
  try {
    const whoResp = await fetch(`${baseUrl}/who`, { signal: AbortSignal.timeout(1000) });
    if (whoResp.ok) {
      const data = await whoResp.json();
      return data.service === 'hippocampus';
    }
    const healthResp = await fetch(`${baseUrl}/health`, { signal: AbortSignal.timeout(1000) });
    if (healthResp.ok) {
      const data = await healthResp.json();
      return data.service === 'hippocampus';
    }
    return false;
  } catch { return false; }
}

async function discoverApiBase() {
  const envUrl = process.env.HIPPOCAMPUS_HTTP_URL;
  if (envUrl) {
    if (/^\d+$/.test(envUrl)) return `http://localhost:${envUrl}/api`;
    return envUrl;
  }
  for (let i = 0; i < PORT_RANGE; i++) {
    const baseUrl = `http://localhost:${DEFAULT_PORT + i}/api`;
    if (await isHippocampusServer(baseUrl)) return baseUrl;
  }
  return null;
}

async function startHippocampusServer() {
  const { spawn } = require('child_process');
  for (const args of [['setsid', HIPPOCAMPUS_BINARY, '--http'], [HIPPOCAMPUS_BINARY, '--http']]) {
    try {
      const proc = spawn(args[0], args.slice(1), { stdio: 'ignore', detached: true });
      proc.unref();
      log("Server spawned", { args });
      await new Promise(r => setTimeout(r, 2000));
      if (!proc.killed) return true;
    } catch { /* try next */ }
  }
  return false;
}

async function ensureApiAvailable() {
  let apiBase = await discoverApiBase();
  if (apiBase) return apiBase;

  const started = await startHippocampusServer();
  if (!started) return null;

  await new Promise(r => setTimeout(r, 5000));
  for (let i = 0; i < 10; i++) {
    apiBase = await discoverApiBase();
    if (apiBase) return apiBase;
    await new Promise(r => setTimeout(r, 2000));
  }
  return null;
}

async function fetchMemories(project) {
  const apiBase = await ensureApiAvailable();
  if (!apiBase) return null;

  try {
    const [projectResp, personalResp, globalResp] = await Promise.all([
      fetch(`${apiBase}/list?scope=project&project=${encodeURIComponent(project)}&limit=${PROJECT_MEMORY_LIMIT}`),
      fetch(`${apiBase}/list?scope=personal&limit=${PERSONAL_MEMORY_LIMIT}`),
      fetch(`${apiBase}/list?scope=global&limit=${GLOBAL_MEMORY_LIMIT}`),
    ]);


    const memories = [];
    if (projectResp.ok)  memories.push(...((await projectResp.json()).memories  || []));
    if (personalResp.ok) memories.push(...((await personalResp.json()).memories || []));
    if (globalResp.ok)   memories.push(...((await globalResp.json()).memories   || []));

    log("Fetched memories", { project, total: memories.length });
    return memories;
  } catch (err) {
    log("Failed to fetch memories", { error: err.message });
    return null;
  }
}

const MEMORY_INJECTION_HEADER =
  "[HIPPOCAMPUS MEMORIES]\nThe following memories are related to the current context.";

function formatMemoriesContext(memories) {
  if (!memories || memories.length === 0) return "";

  const body = memories.map(m => {
    const scopeLine = m.scope ? `Scope: ${m.scope}\n` : "";
    return `Project: ${m.project}\nContext: ${m.context}\n${scopeLine}Content: ${m.content}\nKeywords: ${(m.keywords || []).join(", ")}`;
  }).join("\n---\n");

  return `${MEMORY_INJECTION_HEADER}\n\n${body}`;
}

async function main() {
  const input = await readInput();
  const eventName = input.hook_event_name || "SessionStart";
  const cwd = input.cwd || process.cwd();
  const project = getProjectName(cwd);

  log("inject-memories hook invoked", { eventName, project });

  const memories = await fetchMemories(project);

  if (memories === null) {
    writeOutput(eventName, `<hippocampus_memories>\n⚠️ Could not connect to Hippocampus API. Make sure the server is running.\n</hippocampus_memories>`);
    return;
  }

  if (memories.length === 0) {
    // No memories — inject nothing (silent)
    writeOutput(eventName, undefined);
    return;
  }

  writeOutput(eventName, formatMemoriesContext(memories));
}

main().catch(err => {
  const input_event = "SessionStart"; // safe fallback
  writeOutput(input_event, `<hippocampus_memories>\n❌ Error: ${err.message}\n</hippocampus_memories>`);
});
