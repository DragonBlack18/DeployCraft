# ADR-003 — Plano determinístico antes de executar

Status: Aceito — 2026-10-28.

O planner da Fase 2 retornará dados ordenados por dependência, sem I/O, para permitir dry-run, testes golden e retomada após reinício.
