# ADR-006 — Confiança offline

Status: Aceito — 2026-10-28.

Hash e tamanho do payload são verificados antes de uso; assinatura Ed25519/ECDSA do catálogo e Authenticode do motor serão exigidas para homologação (fase 3). SHA-256 num catálogo não assinado não fornece autenticidade.
