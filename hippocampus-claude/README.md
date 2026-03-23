# Hippocampus Claude Code Plugin

Plugin de memória para Claude Code com injeção automática de memórias e detecção de compactação.

## Requisitos

- Node.js 18+
- Hippocampus MCP server rodando

## Instalação

### Opção 1: Build local

```bash
cd hippocampus-claude
npm install
npm run build
```

### Opção 2: Instalar via Makefile

```bash
make install-hippocampus-claude
```

## Configuração

### 1. Registrar o plugin

No Claude Code, adicione o plugin:

```bash
/plugin install ~/.hippocampus/hippocampus-claude/plugin
```

Ou use o marketplace (quando disponível):

```bash
/plugin marketplace add hippocampus-plugins
```

### 2. Variáveis de Ambiente

Adicione ao seu `~/.zshrc` ou `~/.bashrc`:

```bash
export HIPPOCAMPUS_HTTP_URL="http://localhost:8765/api"
```

## Funcionalidades

### 1. Injeção Automática de Memórias

Quando você inicia uma sessão, o hook `SessionStart`:
- Busca memórias do projeto atual via HTTP API
- Injeta no contexto como `<hippocampus_memories>`
- Visível apenas para o Claude, não polui o chat

### 2. Detecção de Padrões de Memória

O hook `Stop` analisa o transcript da sessão:
- Detecta padrões em português e inglês:
  - "lembre-se", "guarde em memória", "memorize"
  - "remember this", "save to memory"
- Sugere criar memória se padrões forem detectados

### 3. Comandos Slash

#### `/hippocampus load`
Carrega e exibe memórias do projeto atual.

#### `/hippocampus save --content "..." --context "..."`
Salva uma nova memória.

**Parâmetros:**
- `--content` (obrigatório): O que lembrar
- `--context` (obrigatório): Categoria
- `--project` (opcional): Projeto (padrão: diretório atual)
- `--keywords` (opcional): Palavras-chave

**Exemplo:**
```bash
/hippocampus save --content "Uses TypeScript with strict mode" --context "Code Style" --keywords "typescript,strict"
```

#### `/hippocampus search <query>`
Busca memórias por texto (busca semântica).

#### `/hippocampus list [--project <name>]`
Lista todas as memórias ou filtra por projeto.

## Estrutura do Plugin

```
hippocampus-claude/
├── .claude-plugin/
│   └── marketplace.json      # Registro no marketplace
├── plugin/
│   ├── .claude-plugin/
│   │   └── plugin.json       # Manifesto do plugin
│   ├── commands/
│   │   ├── load.md           # /hippocampus load
│   │   ├── save.md           # /hippocampus save
│   │   ├── search.md         # /hippocampus search
│   │   └── list.md           # /hippocampus list
│   ├── hooks/
│   │   └── hooks.json        # Registro de hooks
│   └── scripts/              # Scripts compilados (.cjs)
│       ├── session-start.cjs
│       ├── session-stop.cjs
│       └── command-handler.cjs
├── src/
│   ├── session-start.js      # Hook SessionStart
│   ├── session-stop.js       # Hook Stop
│   └── command-handler.js    # Handler de comandos
├── scripts/
│   └── build.js              # Build script (esbuild)
└── package.json
```

## Como Funciona

### Hook SessionStart

```
1. Claude Code inicia sessão
   ↓
2. Hook recebe input via STDIN (cwd, session_id)
   ↓
3. Busca memórias via HTTP API
   ↓
4. Output via STDOUT com additionalContext
   ↓
5. Claude injeta no system prompt
```

### Hook Stop

```
1. Sessão termina
   ↓
2. Hook recebe transcript_path via STDIN
   ↓
3. Analisa transcript em busca de padrões
   ↓
4. Se detectar padrões de memória, sugere salvar
   ↓
5. Output via STDOUT
```

## Comparação: OpenCode vs Claude Code

| Feature | OpenCode Plugin | Claude Code Plugin |
|---------|----------------|-------------------|
| **Injeção** | `chat.message` hook | `SessionStart` hook |
| **Compactação** | `event: message.updated` com `summary: true` | N/A (Claude gerencia internamente) |
| **Comandos** | MCP tools | Slash commands + MCP tools |
| **Config** | `~/.config/opencode/opencode.json` | `/plugin install` |

## Troubleshooting

### "Could not connect to Hippocampus API"

Verifique se o MCP server está rodando:

```bash
hippocampus --help
```

### Memórias não aparecem

1. Verifique se o projeto tem memórias salvas:
   ```bash
   curl http://localhost:8765/api/list
   ```

2. Verifique o nome do projeto:
   ```bash
   echo $PWD  # O projeto é o último diretório
   ```

### Hooks não executam

Verifique o registro em `plugin/hooks/hooks.json` e reinstale:

```bash
/plugin uninstall hippocampus
/plugin install ~/.hippocampus/hippocampus-claude/plugin
```

## License

MIT
