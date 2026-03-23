# Lessons Learned

## 2026-03-19 | Não criou tasks/ToDo.md e tasks/Lessons.md no início da sessão | Sempre criar diretório tasks/ com arquivos vazios se não existirem antes de começar qualquer trabalho, conforme AGENTS.md

## 2026-03-20 | Servidor hippocampus morre quando executado em background | O MCP server.Run() bloqueia esperando stdin/stdout. Quando executado em background (nohup), o stdin é /dev/null e causa problemas. Solução: adicionar modo HTTP-only com variável HIPPOCAMPUS_HTTP_ONLY para rodar apenas a API HTTP sem MCP.

## 2026-03-20 | Script de instalação falha ao instalar dependências do plugin | A função install_plugin() estava incompleta (faltava código após definir src_plugin). Corrigido copiando a lógica de cópia do install_claude_plugin().

## 2026-03-20 | Plugin auto-start não detecta API corretamente | A função checkApiAvailable verificava resposta HTTP 200 OK no endpoint raiz `/`, mas o servidor não tem handler para raiz. Corrigido verificando endpoint `/api/list` que sempre responde (mesmo com erro 400). Agora qualquer resposta HTTP (não erro de rede) indica API disponível.

## 2026-03-20 | Processo filho morre após spawn | Ao usar child_process.spawn com detached: true, o processo filho pode ser encerrado quando o pai termina. Solução: usar `proc.unref()` e redirecionar stdio para 'ignore'. Além disso, aumentar tempo de espera e retries para garantir que o servidor tenha tempo de iniciar.

## 2026-03-20 | Binário antigo sendo executado mesmo após rebuild | O binário instalado em ~/.local/bin não era atualizado porque estava em uso. Necessário matar processos antes de instalar. Adicionar verificação no Makefile ou script de instalação.

## 2026-03-20 | Health endpoint simplifica verificação de disponibilidade da API | O endpoint /api/list exigia parâmetro project e retornava 400 Bad Request quando não fornecido, complicando a verificação de disponibilidade. Criado endpoint /api/health que sempre retorna 200 OK, simplificando a lógica nos plugins e scripts.

## 2026-03-20 | Servidor HTTP deve encerrar se falhar ao iniciar em modo HTTP-only | O servidor HTTP rodava em goroutine separada; se ListenAndServe falhasse (ex: porta ocupada), o processo principal continuava rodando indefinidamente. Corrigido enviando erro para canal e usando log.Fatal em modo HTTP-only.

## 2026-03-20 | Conflito de portas resolvido com tentativa de portas alternativas e descoberta automática | Quando o servidor hippocampus não consegue usar a porta padrão (8765) em modo auto-start, tenta portas 8766-8774. Plugins escaneiam essas portas para encontrar o servidor, eliminando a necessidade de configuração manual de porta.

## 2026-03-20 | Endpoint /api/who permite identificação confiável do servidor hippocampus | Plugins precisam distinguir hippocampus de outros servidores HTTP. O endpoint /api/who retorna {"service": "hippocampus", ...} permitindo identificação específica. Plugins primeiro tentam /api/who, depois fallback para /api/health para compatibilidade.

## 2026-03-20 | Logging para arquivo permite debugging sem quebrar interface OpenCode | Plugins não podem escrever no stdout/stderr pois quebra a interface OpenCode. Solução: redirecionar logs para arquivo (/tmp/hippocampus-plugin.log) quando HIPPOCAMPUS_DEBUG=true, usando appendFileSync para garantir logs não são perdidos.

## 2026-03-20 | Implementação de escopo (global, personal, project) no hippocampus | Adicionado suporte a três tipos de escopo para memórias: global (acessível por qualquer projeto), personal (usuário específico), project (padrão, compatível com versões anteriores). Campo scope adicionado ao modelo Memory, payload Qdrant, e filtros nas operações de busca/listagem. Backward compatibility garantida: memórias antigas sem scope são tratadas como project.

## 2026-03-21 | Teste MCP vs HTTP - protocolos diferentes | MCP (Model Context Protocol) é diferente de HTTP API. MCP é protocolo stdio para LLMs; HTTP API é para aplicações web/plugins. Ao modificar código MCP, deve-se testar via stdio executando o binário compilado e comunicando-se com ele via stdin/stdout, não apenas testar endpoints HTTP. As ferramentas MCP (create_memory, search_memories, list_memories) devem expor todos os campos necessários (incluindo scope) em seus schemas e handlers.

## 2026-03-21 | Correção de comportamento de scope nos handlers MCP | O handler MCP não deve forçar que buscas sem scope sejam convertidas para scope=project. Agora: search_memories aceita todos parâmetros como opcionais, scope filter é opcional (se omitido, busca em todos scopes). list_memories requer scope obrigatório, project obrigatório apenas quando scope=project, keywords não permitido. API HTTP atualizada para comportamento idêntico.

## 2026-03-21 | Implementação de multi-vectors Qdrant | Substituída replicação de pontos (múltiplos pontos com mesmo memory_id) por multi-vectors do Qdrant (um único ponto com múltiplos vetores nomeados). Cada memória agora gera embeddings para: 1) todos parâmetros combinados (main), 2) cada parâmetro isolado (project, context, content), 3) 5 keywords individuais (keyword1-keyword5). Busca otimizada com SearchBatch para consultas paralelas. Coleção configurada com 10 vetores nomeados. Validação PRD bem-sucedida com testes de criação, listagem e busca para escopos personal, global e project.

## 2026-03-21 | Script de instalação automatiza setup Qdrant+Ollama | Modificado install.py para automaticamente verificar e iniciar serviços Qdrant (via docker-compose) e Ollama (serviço background). Adicionado hook de auto-start (hippocampus-startup-hook.sh) que pode ser adicionado ao shell profile para garantir serviços rodando ao abrir OpenCode. Output do script sugere comando one-liner para fácil configuração.
