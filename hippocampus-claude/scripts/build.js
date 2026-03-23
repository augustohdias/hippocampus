#!/usr/bin/env node
const esbuild = require("esbuild");
const path = require("path");
const fs = require("fs");

const SRC = path.join(__dirname, "..", "src");
const OUT = path.join(__dirname, "..", "plugin", "scripts");

// Ensure output directory exists
if (!fs.existsSync(OUT)) {
  fs.mkdirSync(OUT, { recursive: true });
}

const scripts = [
  "session-start",
  "session-stop",
  "command-handler",
  "user-prompt-hook",
];

async function build() {
  console.log("Building Hippocampus Claude Code plugin...\n");

  for (const script of scripts) {
    const entryPoint = path.join(SRC, `${script}.js`);
    const outFile = path.join(OUT, `${script}.cjs`);

    if (!fs.existsSync(entryPoint)) {
      console.warn(`⚠️  Skipping ${script}: source not found`);
      continue;
    }

    try {
      await esbuild.build({
        entryPoints: [entryPoint],
        bundle: true,
        platform: "node",
        target: "node18",
        format: "cjs",
        outfile: outFile,
        minify: true,
        // Shebang is already in source files
        external: ["fs", "path", "os", "crypto"],
      });
      console.log(`✅ Built ${script}.cjs`);
    } catch (error) {
      console.error(`❌ Error building ${script}:`, error.message);
      process.exit(1);
    }
  }

  console.log("\n✅ Build complete!");
}

build();
