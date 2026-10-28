# Roadmap — DeployCraft

| Fase | Entregas | Gate de aceite |
| --- | --- | --- |
| 0 — Fundamentos | Go, contratos, schema, loader, SHA-256, Actions, ambientes | Quality gate Windows + Linux verde |
| 1 — MVP real | Detecção Windows, driver MSI/EXE, runner, journal | Um pacote instala em VM; segunda rodada pula |
| 2 — Planejamento (LAB) | Perfis, dependências, conflitos, dry-run, lote e resume manual | Testes de lógica e E2E no Actions; RunOnce e VM reiniciada pendentes |
| 3A — Kit-builder (LAB) | add/update, build pasta, keygen e assinatura Ed25519 | CI valida importação, kit assinado, adulteração e chave fixa; **SFX/Authenticode pendentes** |
| 4 — Robustez | Job Objects, retries, rollback, portable, MSIX, secrets | Erros automatizados e VM E2E aprovados |
| 5 — Interface | UI desktop nativa, motion, personalização completa | GUI usa somente `app`; regressão verde |

**Fase 0 ainda não instala software.** Artefatos de CI não são releases homologadas.
