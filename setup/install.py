#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""
Hippocampus MCP + OpenCode Plugin Installer

Installs:
1. hippocampus binary to ~/.local/bin/
2. OpenCode plugin to ~/.hippocampus/plugin/
3. Updates ~/.config/opencode/opencode.json (preserves existing config)
"""

import json
import os
import re
import shutil
import subprocess
import sys
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Optional


# Colors for output
class Colors:
    RED = "\033[0;31m"
    GREEN = "\033[0;32m"
    YELLOW = "\033[1;33m"
    BLUE = "\033[0;34m"
    NC = "\033[0m"  # No Color


def log_info(msg: str):
    print(f"{Colors.BLUE}[INFO]{Colors.NC} {msg}")


def log_success(msg: str):
    print(f"{Colors.GREEN}[SUCCESS]{Colors.NC} {msg}")


def log_warn(msg: str):
    print(f"{Colors.YELLOW}[WARN]{Colors.NC} {msg}")


def log_error(msg: str):
    print(f"{Colors.RED}[ERROR]{Colors.NC} {msg}")


def get_script_dir() -> Path:
    return Path(__file__).parent.resolve()


def get_project_root() -> Path:
    return get_script_dir().parent


def check_project_root():
    """Check if running from project root"""
    if not (get_project_root() / "Makefile").exists():
        log_error("Makefile not found. Are you running from the hippocampus project?")
        sys.exit(1)


def install_binary():
    """Install Go binary to ~/.local/bin"""
    local_bin = Path.home() / ".local" / "bin"
    binary_path = local_bin / "hippocampus"

    # Check if already installed
    if binary_path.exists():
        # Check if it's in PATH
        hippocampus_in_path = shutil.which("hippocampus")
        if hippocampus_in_path and Path(hippocampus_in_path) == binary_path:
            log_info("hippocampus binary already installed at ~/.local/bin/hippocampus")
            return
        elif hippocampus_in_path:
            log_warn(
                f"hippocampus found at {hippocampus_in_path}, will update to ~/.local/bin/hippocampus"
            )

    log_info("Building hippocampus binary...")
    subprocess.run(["make", "build"], cwd=get_project_root(), check=True)

    # Create ~/.local/bin if it doesn't exist
    local_bin.mkdir(parents=True, exist_ok=True)

    log_info("Installing binary to ~/.local/bin/hippocampus...")
    bin_src = get_project_root() / "bin" / "hippocampus"
    shutil.copy2(bin_src, binary_path)
    binary_path.chmod(0o755)

    # Check if ~/.local/bin is in PATH
    path_env = os.environ.get("PATH", "")
    if str(local_bin) not in path_env:
        log_warn("~/.local/bin is not in your PATH")
        log_info("Add this to your ~/.bashrc or ~/.zshrc:")
        print('export PATH="$HOME/.local/bin:$PATH"')

        # Try to add it automatically
        shell_config = None
        if (Path.home() / ".bashrc").exists():
            shell_config = Path.home() / ".bashrc"
        elif (Path.home() / ".zshrc").exists():
            shell_config = Path.home() / ".zshrc"

        if shell_config:
            content = shell_config.read_text()
            if ".local/bin" not in content:
                with open(shell_config, "a") as f:
                    f.write("\n# Added by hippocampus installer\n")
                    f.write('export PATH="$HOME/.local/bin:$PATH"\n')
                log_success(f"Added ~/.local/bin to PATH in {shell_config}")
    else:
        log_success("Binary installed to ~/.local/bin/hippocampus")


def install_plugin():
    """Install OpenCode plugin to ~/.hippocampus/hippocampus-opencode"""
    plugin_dir = Path.home() / ".hippocampus" / "hippocampus-opencode"

    # Check if already installed
    if plugin_dir.exists() and (plugin_dir / "src" / "index.ts").exists():
        log_info("Plugin directory already exists. Updating files...")

    log_info("Installing OpenCode plugin to ~/.hippocampus/hippocampus-opencode...")

    # Create plugin directory
    plugin_dir.mkdir(parents=True, exist_ok=True)

    # Copy plugin files (excluding node_modules)
    src_plugin = get_project_root() / "hippocampus-opencode"

    # Use rsync if available, otherwise use shutil
    if shutil.which("rsync"):
        subprocess.run(
            [
                "rsync",
                "-av",
                "--exclude",
                "node_modules",
                "--exclude",
                "dist",
                f"{src_plugin}/",
                f"{plugin_dir}/",
            ],
            check=True,
        )
    else:
        # Manual copy excluding node_modules and dist
        for item in src_plugin.iterdir():
            if item.name in ("node_modules", "dist"):
                continue
            src_item = item
            dst_item = plugin_dir / item.name

            if item.is_dir():
                if dst_item.exists():
                    shutil.rmtree(dst_item)
                shutil.copytree(src_item, dst_item)
            else:
                shutil.copy2(src_item, dst_item)

    log_success("Plugin installed to ~/.hippocampus/hippocampus-opencode/")


def install_claude_plugin():
    """Install Claude Code plugin to ~/.hippocampus/hippocampus-claude"""
    plugin_dir = Path.home() / ".hippocampus" / "hippocampus-claude"

    log_info("Installing Claude Code plugin to ~/.hippocampus/hippocampus-claude...")

    # Create plugin directory
    plugin_dir.mkdir(parents=True, exist_ok=True)

    # Copy plugin files (excluding node_modules)
    src_plugin = get_project_root() / "hippocampus-claude"

    # Use rsync if available, otherwise use shutil
    if shutil.which("rsync"):
        subprocess.run(
            [
                "rsync",
                "-av",
                "--exclude",
                "node_modules",
                "--exclude",
                "plugin/scripts",
                f"{src_plugin}/",
                f"{plugin_dir}/",
            ],
            check=True,
        )
    else:
        # Manual copy excluding node_modules
        for item in src_plugin.iterdir():
            if item.name in ("node_modules",):
                continue
            src_item = item
            dst_item = plugin_dir / item.name

            if item.is_dir():
                if dst_item.exists():
                    shutil.rmtree(dst_item)
                shutil.copytree(src_item, dst_item)
            else:
                shutil.copy2(src_item, dst_item)

    log_success("Claude Code plugin installed to ~/.hippocampus/hippocampus-claude/")

    # Build the plugin
    log_info("Building Claude Code plugin...")
    if shutil.which("npm"):
        subprocess.run(["npm", "install"], cwd=plugin_dir, check=True)
        subprocess.run(["npm", "run", "build"], cwd=plugin_dir, check=True)
        log_success("Claude Code plugin built successfully")
    elif shutil.which("bun"):
        subprocess.run(["bun", "install"], cwd=plugin_dir, check=True)
        subprocess.run(["bun", "run", "build"], cwd=plugin_dir, check=True)
        log_success("Claude Code plugin built successfully")
    else:
        log_warn(
            "Neither bun nor npm found. Please install dependencies and build manually:"
        )
        print(f"  cd {plugin_dir}")
        print("  npm install && npm run build")
    
    # Register with Claude Code
    register_claude_plugin()


def register_claude_hooks():
    """Register Hippocampus hooks in ~/.claude/settings.json"""
    settings_path = Path.home() / ".claude" / "settings.json"
    scripts_dir = Path.home() / ".claude" / "plugins" / "hippocampus" / "scripts"

    log_info("Registering Hippocampus hooks in ~/.claude/settings.json...")

    # Read existing settings
    settings: Dict[str, Any] = {}
    if settings_path.exists():
        try:
            settings = json.loads(settings_path.read_text())
        except json.JSONDecodeError as e:
            log_warn(f"Could not parse ~/.claude/settings.json, starting fresh: {e}")

    hooks = settings.setdefault("hooks", {})

    # Define hippocampus hooks with absolute paths.
    # inject-memories fires on SessionStart, SubagentStart, and PostCompact.
    # user-prompt-hook fires on UserPromptSubmit to detect memorization patterns.
    inject_cmd = f'node "{scripts_dir}/inject-memories.cjs"'
    nudge_cmd  = f'node "{scripts_dir}/user-prompt-hook.cjs"'
    hippocampus_hooks = {
        "SessionStart":    {"command": inject_cmd, "timeout": 30},
        "SubagentStart":   {"command": inject_cmd, "timeout": 30},
        "PostCompact":     {"command": inject_cmd, "timeout": 30},
        "UserPromptSubmit":{"command": nudge_cmd,  "timeout": 10},
    }

    # Remove stale entries for events that are no longer used
    stale_events = ["Stop"]
    for event in stale_events:
        if event in hooks:
            hooks[event] = [
                g for g in hooks[event]
                if not any("hippocampus" in h.get("command", "") for h in g.get("hooks", []))
            ]
            if not hooks[event]:
                del hooks[event]

    for event, cfg in hippocampus_hooks.items():
        hook_entry = {"hooks": [{"type": "command", "command": cfg["command"], "timeout": cfg["timeout"]}]}
        existing = hooks.setdefault(event, [])

        # Remove any stale hippocampus entries (identified by script path)
        hooks[event] = [
            g for g in existing
            if not any(
                "hippocampus" in h.get("command", "")
                for h in g.get("hooks", [])
            )
        ]
        hooks[event].append(hook_entry)

    settings_path.parent.mkdir(parents=True, exist_ok=True)
    settings_path.write_text(json.dumps(settings, indent=2) + "\n")
    log_success("Hippocampus hooks registered in ~/.claude/settings.json")


def register_claude_plugin():
    """Register Hippocampus MCP server and install plugin with Claude Code"""
    plugin_dir = Path.home() / ".hippocampus" / "hippocampus-claude"
    plugin_path = plugin_dir / "plugin"
    
    log_info("Registering Hippocampus with Claude Code...")
    
    # Check if claude CLI is available
    claude_cmd = shutil.which("claude")
    if not claude_cmd:
        log_warn("Claude CLI not found in PATH. Skipping Claude Code plugin registration.")
        log_info("To manually register, run:")
        print(f"  claude mcp add --scope user hippocampus --transport stdio -- hippocampus")
        print(f"  # Plugin directory: {plugin_path}")
        print(f"  # Use --plugin-dir flag to load plugin: claude --plugin-dir {plugin_path}")
        return
    
    try:
        # Check if MCP server already registered
        result = subprocess.run([claude_cmd, "mcp", "list"], capture_output=True, text=True, timeout=10)
        if "hippocampus" in result.stdout:
            log_info("Hippocampus MCP server already registered")
        else:
            log_info("Registering Hippocampus MCP server...")
            subprocess.run([claude_cmd, "mcp", "add", "--scope", "user", "hippocampus", "--transport", "stdio", "--", "hippocampus"], check=True)
            log_success("Hippocampus MCP server registered")
        
        # Validate plugin manifest
        log_info("Validating Hippocampus plugin manifest...")
        subprocess.run([claude_cmd, "plugin", "validate", str(plugin_path)], check=True)
        log_success("Plugin manifest validated")
        
        # Copy plugin to Claude Code plugins directory for auto-discovery
        claude_plugins_dir = Path.home() / ".claude" / "plugins"
        target_plugin_dir = claude_plugins_dir / "hippocampus"
        if target_plugin_dir.exists():
            log_info("Updating plugin in ~/.claude/plugins/hippocampus...")
            shutil.copytree(plugin_path, target_plugin_dir, dirs_exist_ok=True)
            log_success("Plugin updated in ~/.claude/plugins/hippocampus")
        else:
            log_info("Copying plugin to ~/.claude/plugins/hippocampus for auto-discovery...")
            shutil.copytree(plugin_path, target_plugin_dir)
            log_success("Plugin copied to ~/.claude/plugins/hippocampus")
        
        log_success("Hippocampus plugin registered with Claude Code")
        log_info("Note: The plugin may need to be enabled manually if not auto-discovered.")
        log_info(f"Plugin directory: {plugin_path}")
        log_info(f"To load plugin for a session, use: claude --plugin-dir {plugin_path}")

        # Register hooks in ~/.claude/settings.json
        register_claude_hooks()

    except subprocess.CalledProcessError as e:
        log_warn(f"Failed to register/install plugin: {e}")
        log_info("You may need to run the commands manually:")
        print(f"  claude mcp add --scope user hippocampus --transport stdio -- hippocampus")
        print(f"  # Plugin directory: {plugin_path}")
        print(f"  # Use --plugin-dir flag to load plugin: claude --plugin-dir {plugin_path}")
    except Exception as e:
        log_warn(f"Unexpected error: {e}")


def install_plugin_deps():
    """Install plugin dependencies"""
    plugin_dir = Path.home() / ".hippocampus" / "hippocampus-opencode"

    # Check if node_modules exists
    if (plugin_dir / "node_modules").exists():
        log_info("Plugin dependencies already installed")
        return

    log_info("Installing plugin dependencies...")

    # Try bun first, then npm
    if shutil.which("bun"):
        subprocess.run(["bun", "install"], cwd=plugin_dir, check=True)
        log_success("Plugin dependencies installed with bun")
    elif shutil.which("npm"):
        subprocess.run(["npm", "install"], cwd=plugin_dir, check=True)
        log_success("Plugin dependencies installed with npm")
    else:
        log_error("Neither bun nor npm found. Please install one of them.")
        sys.exit(1)


def strip_jsonc_comments(content: str) -> str:
    """Remove comments from JSONC content"""
    # Remove single-line comments
    content = re.sub(r"//.*$", "", content, flags=re.MULTILINE)
    # Remove multi-line comments
    content = re.sub(r"/\*[\s\S]*?\*/", "", content)
    return content


def update_opencode_config():
    """Update OpenCode configuration (preserves existing config)"""
    config_dir = Path.home() / ".config" / "opencode"
    config_json = config_dir / "opencode.json"
    config_jsonc = config_dir / "opencode.jsonc"

    log_info("Updating OpenCode configuration...")

    # Create config directory if it doesn't exist
    config_dir.mkdir(parents=True, exist_ok=True)

    # Determine which config file to use
    config_path: Optional[Path] = None
    config_type = "json"

    if config_jsonc.exists():
        config_path = config_jsonc
        config_type = "jsonc"
        log_info(f"Using existing JSONC config: {config_path}")
    elif config_json.exists():
        config_path = config_json
        log_info(f"Using existing JSON config: {config_path}")
    else:
        # Create new JSON config
        config_path = config_json
        config_path.write_text("{}\n")
        log_info(f"Created new config: {config_path}")

    # Backup existing config
    if config_path.exists():
        backup_path = config_path.with_suffix(
            config_path.suffix + f".bak.{datetime.now().strftime('%Y%m%d%H%M%S')}"
        )
        shutil.copy2(config_path, backup_path)
        log_info(f"Backup created: {backup_path}")

    # Read and parse existing config
    config: Dict[str, Any] = {}
    raw_content = config_path.read_text()

    try:
        if config_type == "jsonc":
            cleaned = strip_jsonc_comments(raw_content)
            config = json.loads(cleaned)
        else:
            config = json.loads(raw_content)
        log_info("Parsed existing config successfully")
    except json.JSONDecodeError as e:
        log_warn(f"Could not parse existing config, starting fresh: {e}")
        config = {}

    # Preserve existing MCPs and plugins
    existing_mcps = config.get("mcp", {})
    existing_plugins = config.get("plugin", [])
    if not isinstance(existing_plugins, list):
        existing_plugins = []

    # Add/update hippocampus MCP (don't overwrite others)
    plugin_path = f"file://{Path.home()}/.hippocampus/hippocampus-opencode"

    config["mcp"] = {
        **existing_mcps,
        "hippocampus": {"enabled": True, "type": "local", "command": ["hippocampus"]},
    }

    # Add hippocampus plugin if not already present (avoid duplicates)
    plugin_exists = any(
        str(p).endswith(".hippocampus/hippocampus-opencode") or str(p) == plugin_path
        for p in existing_plugins
    )

    if not plugin_exists:
        config["plugin"] = [*existing_plugins, plugin_path]
        log_info("Added hippocampus plugin to config")
    else:
        config["plugin"] = existing_plugins
        log_info("Hippocampus plugin already in config, skipping")

    # Write updated config
    output = json.dumps(config, indent=2)
    config_path.write_text(output + "\n")

    log_success("OpenCode configuration updated (preserved existing MCPs and plugins)")
    log_info(f"MCPs configured: {', '.join(config['mcp'].keys())}")
    log_info(f"Plugins configured: {len(config['plugin'])}")


def verify_installation() -> bool:
    """Verify installation"""
    log_info("Verifying installation...")

    errors = 0

    # Check binary
    hippocampus_path = shutil.which("hippocampus")
    if hippocampus_path:
        log_success(f"hippocampus binary found in PATH: {hippocampus_path}")
        try:
            subprocess.run([hippocampus_path, "--help"], capture_output=True, timeout=5)
            log_success("hippocampus binary is executable")
        except Exception:
            log_warn("Could not verify binary execution")
    else:
        log_error("hippocampus binary not found in PATH")
        errors += 1

    # Check OpenCode plugin directory
    opencode_plugin_dir = Path.home() / ".hippocampus" / "hippocampus-opencode"
    if opencode_plugin_dir.exists():
        log_success(
            "OpenCode plugin directory exists at ~/.hippocampus/hippocampus-opencode/"
        )
    else:
        log_error("OpenCode plugin directory not found")
        errors += 1

    # Check OpenCode plugin files
    if (opencode_plugin_dir / "src" / "index.ts").exists():
        log_success("OpenCode plugin source files found")
    else:
        log_error("OpenCode plugin source files not found")
        errors += 1

    # Check Claude Code plugin directory
    claude_plugin_dir = Path.home() / ".hippocampus" / "hippocampus-claude"
    if claude_plugin_dir.exists():
        log_success("Claude Code plugin directory exists at ~/.hippocampus/hippocampus-claude/")
    else:
        log_info("Claude Code plugin directory not found (optional)")

    # Check Claude Code plugin built files
    if (claude_plugin_dir / "plugin" / "scripts").exists():
        log_success("Claude Code plugin built files found")
    else:
        log_info("Claude Code plugin built files not found (optional)")

    # Check OpenCode config
    config_dir = Path.home() / ".config" / "opencode"
    config_file = config_dir / "opencode.json"
    config_filec = config_dir / "opencode.jsonc"

    if config_file.exists() or config_filec.exists():
        log_success("OpenCode config found")

        # Check which file exists and read it
        config_path = config_filec if config_filec.exists() else config_file
        config_content = config_path.read_text()

        if "hippocampus" in config_content:
            log_success("Hippocampus MCP configured in OpenCode")
        else:
            log_warn("Hippocampus MCP not found in OpenCode config")

        if ".hippocampus/hippocampus-opencode" in config_content:
            log_success("Hippocampus OpenCode plugin configured in OpenCode")
        else:
            log_warn("Hippocampus OpenCode plugin not found in OpenCode config")
    else:
        log_error("OpenCode config not found")
        errors += 1

    return errors == 0


def print_next_steps():
    """Print next steps"""
    project_root = get_project_root()
    print()
    print(f"{Colors.GREEN}========================================{Colors.NC}")
    print(f"{Colors.GREEN}  Hippocampus Installation Complete!   {Colors.NC}")
    print(f"{Colors.GREEN}========================================{Colors.NC}")
    print()
    print("Next steps:")
    print()
    print(f"1. {Colors.YELLOW}Start required services:{Colors.NC}")
    print("   You MUST ensure Qdrant and Ollama are running before using hippocampus.")
    print()
    print("   Start Qdrant (from project directory):")
    print(f"   {Colors.BLUE}cd {project_root} && docker compose up -d qdrant{Colors.NC}")
    print()
    print("   Start Ollama (in a separate terminal):")
    print(f"   {Colors.BLUE}ollama serve{Colors.NC}")
    print()
    print("   Note: Qdrant is configured with 'restart: unless-stopped', so it will")
    print("   automatically start on system reboot after the first manual start.")
    print()
    print(f"2. {Colors.YELLOW}Pull the embedding model (first time only):{Colors.NC}")
    print(f"   {Colors.BLUE}ollama pull qwen3-embedding:4b{Colors.NC}")
    print()
    print(f"3. {Colors.YELLOW}Restart OpenCode{Colors.NC} to load the new plugin")
    print()
    print(f"4. {Colors.YELLOW}Test the installation:{Colors.NC}")
    print("   opencode -c  # Should show hippocampus in tools list")
    print()
    print(f"5. {Colors.GREEN}Done!{Colors.NC}")
    print()
    print("Files installed:")
    print("  - Binary: ~/.local/bin/hippocampus")
    print("  - OpenCode Plugin: ~/.hippocampus/hippocampus-opencode/")
    print("  - Claude Code Plugin: ~/.hippocampus/hippocampus-claude/")
    print("  - Config: ~/.config/opencode/opencode.json")
    print()
    print(f"{Colors.YELLOW}Using external services?{Colors.NC}")
    print("  Configure these environment variables:")
    print("  - QDRANT_HOST (default: localhost:6334)")
    print("  - OLLAMA_MODEL (default: qwen3-embedding:4b)")
    print()


def plugin_only_mode():
    """Plugin-only mode (called from Makefile)"""
    log_info("Running in plugin-only mode...")
    install_plugin()
    install_plugin_deps()
    log_success("Plugin installation complete!")


def config_only_mode():
    """Config-only mode (called from Makefile)"""
    log_info("Running in config-only mode...")
    update_opencode_config()
    log_success("Configuration updated!")


def claude_plugin_only_mode():
    """Claude Code plugin-only mode (called from Makefile)"""
    log_info("Running in Claude plugin-only mode...")
    install_claude_plugin()  # calls register_claude_plugin -> register_claude_hooks
    log_success("Claude Code plugin installation complete!")


def main():
    """Main installation flow"""
    # Check for flags
    if len(sys.argv) > 1:
        if sys.argv[1] == "--plugin-only":
            plugin_only_mode()
            return
        elif sys.argv[1] == "--config-only":
            config_only_mode()
            return
        elif sys.argv[1] == "--claude-plugin-only":
            claude_plugin_only_mode()
            return
        elif sys.argv[1] in ("--yes", "-y"):
            # Continue with full install, skip prompts
            pass

    print(f"{Colors.BLUE}")
    print("========================================")
    print("  Hippocampus MCP + Plugin Installer  ")
    print("========================================")
    print(f"{Colors.NC}")
    print()

    check_project_root()

    # Check what's already installed
    binary_installed = False
    plugin_installed = False

    local_bin = Path.home() / ".local" / "bin"
    binary_path = local_bin / "hippocampus"
    plugin_dir = Path.home() / ".hippocampus" / "hippocampus-opencode"

    if binary_path.exists():
        hippocampus_in_path = shutil.which("hippocampus")
        if hippocampus_in_path and Path(hippocampus_in_path) == binary_path:
            binary_installed = True

    if plugin_dir.exists() and (plugin_dir / "src" / "index.ts").exists():
        plugin_installed = True

    if binary_installed and plugin_installed:
        log_info("Everything appears to be already installed.")
        # Skip prompt if --yes flag is passed
        if len(sys.argv) == 1 or sys.argv[1] not in ("--yes", "-y"):
            response = input("Reinstall anyway? [y/N] ")
            if response.lower() != "y":
                log_warn("Installation cancelled")
                return
        else:
            log_info("Reinstalling with --yes flag...")

    print("This will install:")
    print("  1. hippocampus binary to ~/.local/bin/")
    print("  2. OpenCode plugin to ~/.hippocampus/hippocampus-opencode/")
    print("  3. Claude Code plugin to ~/.hippocampus/hippocampus-claude/")
    print("  4. Register Hippocampus MCP server with Claude Code")
    print("  5. Update ~/.config/opencode/opencode.json (preserving existing config)")
    print()

    # Skip prompt if --yes flag is passed
    if len(sys.argv) == 1 or sys.argv[1] not in ("--yes", "-y"):
        response = input("Continue? [y/N] ")
        if response.lower() != "y":
            log_warn("Installation cancelled")
            return

    print()

    install_binary()
    print()

    install_plugin()
    print()

    install_plugin_deps()
    print()

    install_claude_plugin()
    print()

    register_claude_plugin()
    print()

    update_opencode_config()
    print()

    # Remind about services
    log_info("Remember to start Qdrant and Ollama before using hippocampus!")
    print()

    if verify_installation():
        print_next_steps()
        log_success("Installation completed successfully!")
    else:
        print()
        log_error(
            "Installation completed with errors. Please review the messages above."
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
