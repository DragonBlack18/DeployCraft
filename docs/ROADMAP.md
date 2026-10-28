# Roadmap — DeployCraft

| Fase | Entregas | Gate de aceite |
| --- | --- | --- |
| 0 — Fundamentos | Go, contratos, schema, loader, SHA-256, Actions, ambientes | Quality gate Windows + Linux verde |
| 1 — MVP real | Detecção Windows, driver MSI/EXE, runner, journal | Um pacote instala em VM; segunda rodada pula |
| 2 — Planejamento | Perfil, dependências, conflitos, dry-run, reboot, resume | Plano determinístico e VM reiniciada retoma |
| 3 — Kit-builder | add, update, build, pasta/SFX, assinatura de catálogo | Adicionar software sem código; adulteração recusada |
| 4 — Robustez | Job Objects, retries, rollback, portable, MSIX, secrets | Erros automatizados e VM E2E aprovados |
| 5 — Interface | UI desktop nativa, motion, personalização completa | GUI usa somente `app`; regressão verde |

**Fase 0 ainda não instala software.** Artefatos de CI não são releases homologadas.
