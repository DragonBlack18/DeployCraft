# ADR-005 — Journal append-only

Status: Aceito — 2026-10-28.

Nas fases 1 e 2, cada instalação será registrada em JSON Lines com snapshot atômico. A retomada reconstruirá o plano em vez de confiar num cursor antigo. Fase 0 não registra instalações fictícias.
