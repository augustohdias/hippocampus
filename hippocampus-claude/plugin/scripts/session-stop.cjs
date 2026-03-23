#!/usr/bin/env node
var b=Object.create;var r=Object.defineProperty;var a=Object.getOwnPropertyDescriptor;var c=Object.getOwnPropertyNames;var m=Object.getPrototypeOf,u=Object.prototype.hasOwnProperty;var p=(e,n,t,o)=>{if(n&&typeof n=="object"||typeof n=="function")for(let i of c(n))!u.call(e,i)&&i!==t&&r(e,i,{get:()=>n[i],enumerable:!(o=a(n,i))||o.enumerable});return e};var d=(e,n,t)=>(t=e!=null?b(m(e)):{},p(n||!e||!e.__esModule?r(t,"default",{value:e,enumerable:!0}):t,e));var k=process.env.HIPPOCAMPUS_HTTP_URL||"http://localhost:8765/api",f=[/\blembre-se\b/i,/\bse lembre\b/i,/\bmemorize\b/i,/\bguarde\b/i,/\bguarde em memória\b/i,/\bguarde em memoria\b/i,/\bcrie uma memória\b/i,/\bcrie memória\b/i,/\bcrie memorias\b/i,/\bcrie uma memoria\b/i,/\bcrie memorías\b/i,/\bremember\b/i,/\bsave\b/i,/\bstore\b/i,/\bnote\b/i,/\brecord\b/i,/\bdon't forget\b/i,/\bdo not forget\b/i,/\bmake sure to remember\b/i,/\bthis is important\b/i,/\bkeep in mind\b/i,/\bkeep this in mind\b/i,/\bremember this\b/i,/\bsave this to memory\b/i,/\bmake a note\b/i,/\balways use\b/i,/\bnever use\b/i,/\bprefer\b/i,/\bpreferred\b/i,/\bby convention\b/i,/\bconvention\b/i,/\bstandard practice\b/i,/\bproject uses\b/i,/\bwe use\b/i,/\bour setup\b/i,/\bour stack\b/i,/\bconfigured to\b/i,/\bconfigured for\b/i,/\bsetup to\b/i,/\bthe key is\b/i,/\bthe trick is\b/i,/\bimportant note\b/i,/\bworkaround\b/i,/\bsolution for\b/i,/\bfix for\b/i];function s(e){console.log(JSON.stringify(e))}async function h(){return new Promise(e=>{let n="";process.stdin.setEncoding("utf8"),process.stdin.on("data",t=>{n+=t}),process.stdin.on("end",()=>{try{e(n.trim()?JSON.parse(n):{})}catch{e({})}})})}async function l(e){try{return(await import("fs")).readFileSync(e,"utf8")}catch{return""}}function g(e){let n=[];for(let t of f){let o=e.match(t);o&&n.push(o[0])}return n}async function y(){let e=await h(),n=e.transcript_path,t="";if(n){let o=await l(n),i=g(o);i.length>0&&(t=`

<hippocampus_nudge>
\u{1F9E0} **Memory Creation Detected**

`,t+=`This conversation contains patterns suggesting you want to remember something:
`,t+=`- Found: "${i.join('", "')}"

`,t+=`**Guidelines for saving memories:**
`,t+=`- Extract key information and save as concise, searchable memories
`,t+=`- If investigating, save only conclusions about what needs investigation
`,t+=`- When examining code, store info about core components (business rules, execution flow)
`,t+=`- Prefer multiple small memories instead of one big memory

`,t+="**To save this as a memory, use the `hippocampus` MCP tool:**\n",t+=`\`\`\`json
{
  "project": "${e.cwd?e.cwd.split("/").pop():"current-project"}",
  "content": "Key information to remember",
  "context": "Category (e.g., Code Patterns, Project Setup)",
  "keywords": "keyword1, keyword2"
}
\`\`\`

`,t+=`*The user explicitly asked you to remember. Do not skip this step.*
`,t+="</hippocampus_nudge>")}s({continue:!0,additionalContext:t||void 0})}y().catch(e=>{s({continue:!0})});
