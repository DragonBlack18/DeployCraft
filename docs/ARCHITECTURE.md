# Arquitetura do DeployCraft

Referência normativa: **Instalador Offline de Software — Documentação Técnica** (106 páginas, 2026-10-15).

## Invariantes
1. Motor Go, catálogo JSON e payloads são separados.
2. Sem instaladores de terceiros dentro do executável ou do Git.
3. Nenhuma lógica especial por nome de software no motor: somente contratos.
4. Validar -> detectar -> planejar -> verificar SHA-256 -> instalar -> confirmar por detecção.
5. O erro de detecção nunca se converte em "ausente".
6. GUI desacoplada dos adaptadores do Windows. Sem regra de instalação na interface.
7. Todos os bugs exigem testes de regressão; releases exigem VM Windows limpa.

## Divisão por módulos
- `src/domain`: tipos e comparações puras.
- `src/catalog`: validação do índice, manifestos e verificações de hashes sem executar processos.
- `src/planner`: planejamento determinístico (fase 2).
- `src/detect`, `src/verify`, `src/drivers`, `src/runner`, `src/journal`, `src/platform`: adaptadores reais e falsos (fases 1–4).
- `src/app`: casos de uso (fase 1).
- `ui/`: desktop com motion design e customização (fase 5).
- `cmd/deploycraft`: executável do motor.
- `cmd/kit-builder`: CLI de preparação e verificação do kit.

## Estado atual, Fase 0
Existem loader estrito, contratos de modelos, comparação numérica de versões, detecção de ciclos, verificação SHA-256 em streaming, CLIs de inspeção e quality gate automatizado.

**Não existem ainda** a instalação real, detecção no registro, execução com Job Objects, journal, staging, assinatura, SFX, retomada, rollback e GUI. Este README e os contratos não substituem esses módulos.
