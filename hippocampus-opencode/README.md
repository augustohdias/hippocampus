# hippocampus-opencode

Plugin do OpenCode para integração com o **Hippocampus MCP** - sistema de memórias persistentes para agentes de IA.

## Funcionalidades

### 1. Injeção Automática de Contexto

Na primeira mensagem de cada sessão, o plugin:
- Busca memórias relevantes usando `project` e contexto da mensagem
- Se não encontrar memórias relevantes, lista as memórias mais recentes
- Injeta o contexto na conversa (invisível para o usuário)

### 2. Detecção de Padrões de Memorização

Intercepta todas as mensagens do usuário buscando padrões como:
- "lembre que...", "salve isso...", "não esqueça..."
- "sempre use...", "nunca use...", "preferimos..."
- "o projeto usa...", "nosso setup...", "configurado para..."

Quando detectado, injeta instrução para o agente usar a ferramenta `hippocampus` para memorizar.

### 3. Ferramenta `hippocampus`

Disponibiliza três modos:

#### `add` - Criar memória
```json
{
  "mode": "add",
  "project": "my-app",
  "content": "Usa Bun como runtime. Todos os scripts devem usar 'bun run'",
  "context": "Project Setup",
  "keywords": "bun, runtime, setup, scripts"
}
```

#### `search` - Buscar memórias
```json
{
  "mode": "search",
  "project": "my-app",
  "context": "Setup",
  "keywords": "bun",
  "limit": 5
}
```

#### `list` - Listar memórias
```json
{
  "mode": "list",
  "project": "my-app",
  "keywords": "bun",
  "limit": 100
}
```

## Instalação

### Método Recomendado: Script de Instalação

A partir do diretório do projeto hippocampus:

```bash
# Instala tudo: binário + plugin + configura OpenCode
make install

# Ou com auto-yes (para CI/CD)
make install-yes
```

O script de instalação irá:
1. Compilar e instalar o binário `hippocampus` em `~/.local/bin/`
2. Copiar o plugin para `~/.hippocampus/plugin/`
3. Instalar dependências do plugin (bun/npm)
4. Atualizar `~/.config/opencode/opencode.json` preservando configurações existentes

### Método Manual

1. **Instale o binário MCP:**
```bash
make build
make install-binary
```

2. **Instale o plugin:**
```bash
make install-plugin
```

3. **Configure o OpenCode manualmente:**

Adicione ao seu `~/.config/opencode/opencode.json`:
```json
{
  "mcp": {
    "hippocampus": {
      "enabled": true,
      "type": "local",
      "command": ["hippocampus"]
    }
  },
  "plugin": [
    "file://~/.hippocampus/plugin"
  ]
}
```

## Pré-requisitos

1. **Hippocampus MCP Server** instalado e configurado:
   ```bash
   cd /path/to/hippocampus
   make build
   make dev  # Inicia Qdrant + Ollama + Hippocampus
   ```

2. **OpenCode** configurado para usar o Hippocampus como MCP server:

   Adicione ao `~/.config/opencode/opencode.json`:
   ```json
   {
     "mcp": {
       "hippocampus": {
         "command": "/path/to/hippocampus/bin/hippocampus",
         "args": ["--stdio"]
       }
     }
   }
   ```

## Configuração

O plugin funciona com configurações padrão. Para personalizar, crie `~/.config/opencode/hippocampus.json`:

```json
{
  "similarityThreshold": 0.6,
  "maxMemories": 5,
  "maxProjectMemories": 10,
  "keywordPatterns": ["log this", "write down"],
  "hippocampusBinary": "/path/to/hippocampus/bin/hippocampus"
}
```

## Uso

### Exemplo 1: Memorização Automática

```
Você: Lembre que este projeto usa Bun ao invés de Node.js

Agente: [Detecta padrão de memorização]
[Usa ferramenta hippocampus para salvar]
Memória salva com sucesso!
```

### Exemplo 2: Contexto Automático

```
Você: Como configuro os scripts de build?

Agente: [Recebe contexto injetado automaticamente]
[HIPPOCAMPUS MEMORIES]
[100%] Project: my-app
Context: Project Setup
Content: Usa Bun como runtime. Scripts: "bun run build"
Keywords: bun, runtime, setup, scripts

Baseado nas memórias do projeto, os scripts devem usar Bun...
```

### Exemplo 3: Busca Manual

```
Você: Quais são as convenções de código deste projeto?

Agente: [Usa ferramenta hippocampus search]
{
  "mode": "search",
  "project": "my-app",
  "keywords": "convention, code, style"
}

Encontrei 2 memórias relevantes:
1. [95%] Code Style: Usa Prettier com as configurações padrão...
2. [87%] Architecture: Segue Clean Architecture com 4 camadas...
```

## Desenvolvimento

```bash
cd plugin/
bun install
bun run build
bun run typecheck
```

### Test Local

1. Configure o plugin no OpenCode apontando para o diretório local:
```json
{
  "plugin": ["file:///path/to/hippocampus/plugin"]
}
```

2. Reinicie o OpenCode

3. Verifique os logs:
```bash
tail -f ~/.cache/opencode/logs/*.log
```

## Estrutura do Projeto

```
plugin/
├── package.json              # Dependências e scripts
├── tsconfig.json            # Configuração TypeScript
├── src/
│   ├── index.ts             # Plugin principal
│   └── services/
│       ├── client.ts        # Cliente Hippocampus MCP
│       ├── patterns.ts      # Detecção de padrões
│       ├── context.ts       # Formatação de contexto
│       └── logger.ts        # Logging estruturado
└── README.md                # Este arquivo
```

## Padrões de Memorização

O plugin detecta automaticamente estes padrões:

### Explícitos
- "lembre que...", "salve isso...", "memorize..."
- "não esqueça...", "guarde isso..."

### Implícitos
- "sempre use...", "nunca use...", "preferimos..."
- "o projeto usa...", "nosso setup...", "configurado para..."
- "por convenção...", "padrão do projeto..."

### Declarações de Conhecimento
- "a chave é...", "o truque é...", "nota importante..."
- "workaround para...", "solução para..."

## Logs

Os logs do plugin são escritos via `client.app.log()` do OpenCode SDK.

Para visualizar:
```bash
# Logs do OpenCode
tail -f ~/.cache/opencode/logs/*.log

# Ou via comando do próprio OpenCode
opencode --debug
```

## Troubleshooting

### Plugin não carrega
- Verifique se o caminho no `opencode.json` está correto
- Execute `bun install` no diretório do plugin
- Reinicie o OpenCode

### Memórias não são encontradas
- Verifique se o Hippocampus MCP está configurado no OpenCode
- Teste o servidor MCP diretamente: `./bin/hippocampus --stdio`
- Verifique se há memórias no Qdrant

### Contexto não é injetado
- Verifique os logs para erros na busca de memórias
- Confirme que o `project` está sendo extraído corretamente
- Ajuste `similarityThreshold` se necessário

## Licença

MIT
