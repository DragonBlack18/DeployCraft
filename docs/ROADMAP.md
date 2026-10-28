# Roadmap — DeployCraft

| Fase | Entregas | Gate de aceite |
| --- | --- | --- |
| 0 — Fundamentos | Go, contratos, schema, loader, SHA-256, Actions, ambientes | Quality gate Windows + Linux verde |
| 1 — MVP real | Detecção Windows, driver MSI/EXE, runner, journal | Um pacote instala em VM; segunda rodada pula |
| 2 — Planejamento (LAB) | Perfis, dependências, conflitos, dry-run, lote e resume manual | Testes de lógica e E2E no Actions; RunOnce e VM reiniciada pendentes |
| 3A — Kit-builder (LAB) | add/update, build pasta, keygen e assinatura Ed25519 | CI valida importação, kit assinado, adulteração e chave fixa |
| 3B — SFX (LAB) | arquivo único com índice, PackSource em streaming, extração seletiva | CI valida o PE Windows e integridade; **Authenticode e reboot real pendentes** |
| 4A — Retry controlado (LAB) | retryCodes, 15/30/60 s, cancelamento, redetecção e journal | CI Windows + Linux, sequência 1618, falhas e E2E de EXE de teste |
| 4B–4D — Robustez | Job Objects, rollback (atomic), portable/MSIX, desinstalação, secrets | Testes Windows de árvore, zip slip, rollback e VM após reboot |
| 5 — Interface | UI desktop nativa, motion, personalização completa | GUI usa somente `app`; regressão verde |

**As Fases 1–3B permanecem LAB.** Artefatos de CI não são releases corporativas homologadas.
