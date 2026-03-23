.PHONY: all build build-opencode-plugin build-hippocampus-claude run test clean docker-up docker-down install install-binary install-plugin install-hippocampus-claude configure-opencode uninstall setup dev

all: build build-opencode-plugin build-hippocampus-claude

build:
	go build -o bin/hippocampus mcp/cmd/hippocampus/main.go

build-opencode-plugin:
	(cd hippocampus-opencode && bun run build) 2>/dev/null || (echo "Bun not found, trying npm..." && cd hippocampus-opencode && npm run build)

build-hippocampus-claude:
	(cd hippocampus-claude && npm run build)

# Install everything: binary + OpenCode plugin + Claude Code plugin + update configs
install: build
	python3 setup/install.py

# Install everything with auto-yes (for CI/CD)
install-yes: build
	python3 setup/install.py --yes

# Install only the binary to ~/.local/bin
install-binary: build
	mkdir -p ~/.local/bin
	cp ./bin/hippocampus ~/.local/bin/
	chmod +x ~/.local/bin/hippocampus
	@echo "Binary installed to ~/.local/bin/hippocampus"

# Install only the OpenCode plugin to ~/.hippocampus/hippocampus-opencode
install-plugin:
	python3 setup/install.py --plugin-only

# Install Claude Code plugin
install-hippocampus-claude:
	python3 setup/install.py --claude-plugin-only

# Update OpenCode configuration only
configure-opencode:
	python3 setup/install.py --config-only

# Uninstall everything
uninstall:
	@echo "Uninstalling Hippocampus..."
	rm -f ~/.local/bin/hippocampus
	rm -rf ~/.hippocampus
	@echo "Note: OpenCode config not modified. Remove manually if needed:"
	@echo "  Edit ~/.config/opencode/opencode.json"
	@echo "  Remove 'hippocampus' from mcp and plugin sections"
	@echo "Uninstall complete"

run: build
	./bin/hippocampus

test:
	go test ./...

clean:
	rm -rf bin/
	rm -rf hippocampus-opencode/dist/
	(cd hippocampus-claude && npm run clean) 2>/dev/null || true
	rm ~/.local/bin/hippocampus

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

setup:
	docker-compose up -d qdrant
	@echo "Waiting for Qdrant to start..."
	sleep 5
	@echo "Installing Ollama model if not present..."
	ollama pull qwen3-embedding:4b 2>/dev/null || echo "Ollama not installed or model already present"

dev: setup run
