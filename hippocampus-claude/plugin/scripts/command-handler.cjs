#!/usr/bin/env node
var u=process.env.HIPPOCAMPUS_BINARY||"hippocampus",w=8765,g=10;function d(){let o=process.cwd().split("/").filter(t=>t&&t!=="opencode");return o.length>0?o[o.length-1]:"default"}async function y(e){try{let o=`${e}/who`,t=new AbortController,s=setTimeout(()=>t.abort(),1e3),c=await fetch(o,{signal:t.signal});if(clearTimeout(s),c.ok)return(await c.json()).service==="hippocampus";let a=`${e}/health`,n=new AbortController,r=setTimeout(()=>n.abort(),1e3),i=await fetch(a,{signal:n.signal});return clearTimeout(r),i.ok?(await i.json()).service==="hippocampus":!1}catch{return!1}}async function v(){let e=process.env.HIPPOCAMPUS_HTTP_URL;if(e)try{let o=new URL(e);return e}catch{return/^\d+$/.test(e)?`http://localhost:${e}/api`:e}for(let o=0;o<g;o++){let s=`http://localhost:${w+o}/api`;try{if(await y(s))return s}catch{}}return null}async function l(){return await v()}async function m(){return await l()!==null}async function $(){try{let{spawn:e}=require("child_process"),o=e("setsid",[u,"--http"],{stdio:"ignore",detached:!0});return o.unref(),await new Promise(s=>setTimeout(s,2e3)),!o.killed}catch{try{let{spawn:e}=require("child_process"),o=e(u,["--http"],{stdio:"ignore",detached:!0});return o.unref(),await new Promise(s=>setTimeout(s,2e3)),!o.killed}catch{return!1}}}async function p(){if(await m())return!0;if(!await $())return!1;await new Promise(o=>setTimeout(o,5e3));for(let o=0;o<10;o++){if(await m())return!0;await new Promise(t=>setTimeout(t,2e3))}return!1}async function P(e){if(!await p())return null;let t=await l();if(!t)return null;try{let s=`${t}/search?project=${encodeURIComponent(e)}&limit=10`,c=await fetch(s);if(c.ok){let r=await c.json();if(r.results&&r.results.length>0)return r.results.map(i=>i.memory)}let a=`${t}/list?limit=10`,n=await fetch(a);return n.ok?(await n.json()).memories||[]:[]}catch{return null}}function x(e){if(e.length===0)return"\u{1F4ED} No memories found for this project.";let o=`\u{1F9E0} **Hippocampus Memories** (${e.length} found)

`;return e.forEach((t,s)=>{o+=`**${s+1}. ${t.project}** \u2014 ${t.context}
`,o+=`   ${t.content}
`,o+=`   _Keywords: ${t.keywords.join(", ")}_

`}),o.trim()}async function b(){let e=d();console.log(`\u{1F50D} Loading memories for project: ${e}
`);let o=await P(e);o===null&&(console.log("\u26A0\uFE0F  Could not connect to Hippocampus API."),console.log("   Make sure the MCP server is running: `hippocampus`"),process.exit(1)),console.log(x(o)),o.length>0&&console.log(`
\u{1F4A1} These memories are now available in your session context.`)}async function h(e){let o={};for(let t=0;t<e.length;t++)e[t]==="--project"&&e[t+1]?o.project=e[++t]:e[t]==="--content"&&e[t+1]?o.content=e[++t]:e[t]==="--context"&&e[t+1]?o.context=e[++t]:e[t]==="--keywords"&&e[t+1]&&(o.keywords=e[++t]);o.content||(console.log("\u274C Missing required parameter: --content"),console.log(`
Usage:`),console.log("  /hippocampus save --content <text> --context <ctx> [--project <name>] [--keywords <kws>]"),console.log(`
Example:`),console.log('  /hippocampus save --content "Uses TypeScript with strict mode" --context "Code Style" --keywords "typescript,strict"'),process.exit(1)),console.log(`\u{1F4DD} Memory ready to save:
`),console.log(`**Project:** ${o.project||d()}`),console.log(`**Context:** ${o.context||"General"}`),console.log(`**Content:** ${o.content}`),o.keywords&&console.log(`**Keywords:** ${o.keywords}`),console.log("\n\u2705 To save this memory, the MCP tool `hippocampus_create_memory` will be used."),console.log("   The memory will be stored in Qdrant with an embedding for semantic search.")}async function A(e){e||(console.log("\u274C Please provide a search query"),console.log("Usage: /hippocampus search <query>"),process.exit(1)),await p()||(console.log("\u274C Hippocampus API unavailable and server could not be started"),process.exit(1));let t=await l();t||(console.log("\u274C Hippocampus API base URL not found"),process.exit(1));try{let s=`${t}/search?query=${encodeURIComponent(e)}&limit=10`,c=await fetch(s);c.ok||(console.log("\u26A0\uFE0F  Could not connect to Hippocampus API."),process.exit(1));let a=await c.json();if(a.results.length===0){console.log(`\u{1F4ED} No memories found matching "${e}"`);return}console.log(`\u{1F50D} Search results for "${e}" (${a.results.length} found):
`),a.results.forEach((n,r)=>{console.log(`**${r+1}.** [Score: ${(n.score*100).toFixed(0)}%] ${n.memory.project} \u2014 ${n.memory.context}`),console.log(`   ${n.memory.content}`),console.log(`   _Keywords: ${n.memory.keywords.join(", ")}_
`)})}catch{console.log("\u26A0\uFE0F  Error searching memories"),process.exit(1)}}async function j(e){await p()||(console.log("\u274C Hippocampus API unavailable and server could not be started"),process.exit(1));let t=await l();t||(console.log("\u274C Hippocampus API base URL not found"),process.exit(1));try{let s=`${t}/list?limit=10`;e&&(s+=`&project=${encodeURIComponent(e)}`);let c=await fetch(s);c.ok||(console.log("\u26A0\uFE0F  Could not connect to Hippocampus API."),process.exit(1));let a=await c.json();if(a.memories.length===0){console.log("\u{1F4ED} No memories found.");return}console.log(`\u{1F4DA} All memories (${a.memories.length} total, limited to 10 most recent):
`),a.memories.forEach((n,r)=>{console.log(`**${r+1}. ${n.project}** \u2014 ${n.context}`),console.log(`   ${n.content}`),console.log(`   _Keywords: ${n.keywords.join(", ")}_
`)})}catch{console.log("\u26A0\uFE0F  Error listing memories"),process.exit(1)}}async function k(){let e=process.argv.slice(2),o=e[0]||"load",t=e.slice(1);switch(o){case"load":await b();break;case"add":await h(t);break;case"save":await h(t);break;case"search":await A(t.join(" "));break;case"list":let s=t.find(a=>a.startsWith("--project")),c=s?s.split("=")[1]||t[t.indexOf("--project")+1]:void 0;await j(c);break;case"--help":case"-h":console.log(`
\u{1F9E0} Hippocampus \u2014 Memory System for Claude Code

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
`);break;default:console.log(`\u274C Unknown command: ${o}`),console.log("Run /hippocampus --help for usage."),process.exit(1)}}k().catch(e=>{console.error("Error:",e),process.exit(1)});
