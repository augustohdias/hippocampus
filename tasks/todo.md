# TODO - Hippocampus Bug Fixes

## Bugs Identificados (2026-03-22)

- [ ] **[ALTA]** HTTP API `/api/create` retorna `id` como `uint64` numérico → precision loss para IDs > 2^53 (`main.go:341`)
- [ ] **[ALTA]** HTTP API `/api/delete` recebe `memory_id` como `uint64` via JSON → precision loss (`main.go:357-365`)
- [ ] **[MÉDIA]** Race condition em `tryStartServer`: listener fechado antes do `ListenAndServe` (`main.go:60-71`)
- [ ] **[MÉDIA]** `rand.Seed` depreciado e não thread-safe em `generateID` (`service.go:346`)
- [ ] **[MÉDIA]** `ListMemories` no Qdrant não pagina — scroll único pode perder memórias além do limit (`client.go:600-603`)
- [ ] **[BAIXA]** Ollama host não configurável via env var — primeiro parâmetro de `NewClient` ignorado (`ollama/client.go:14`)
- [ ] **[BAIXA]** Score threshold hardcoded em `0.65` — não configurável (`service.go:254`)
- [ ] **[BAIXA]** Bubble sort O(n²) em `SearchMemories` — deveria usar `sort.Slice` (`service.go:288-294`)
- [ ] **[TRIVIAL]** `getEnvInt` definido mas nunca usado — dead code (`config.go:35-42`)

## Notas

- Bugs #1 e #2 são o mesmo problema que foi corrigido no MCP (float64 precision loss), mas ainda existem na HTTP API
- Bug #3 é uma race condition clássica de TOCTOU (time-of-check-time-of-use) em portas TCP
- Bug #4 foi introduzido antes do Go 1.20; desde então `rand` é auto-inicializado com entropia segura
