# DeployCraft

**DeployCraft** é um projeto de instalador universal de software para Windows 10/11 x64, orientado à execução **offline**, com um motor leve e um catálogo extensível.

> Estado do projeto: **Fase 0 — Fundamentos**. Ainda não é um instalador pronto para uso operacional.

## Princípios inegociáveis
- **Motor, catálogo e payloads separados.** Nenhum instalador de terceiros será incorporado ao executável ou versionado no Git.
- **Offline no destino.** Os instaladores devem existir no kit local e ter origem e integridade verificáveis.
- **Plano antes da ação.** Nenhuma instalação sem pré-checagens, detecção e confirmação.
- **Idempotência.** Verificar antes e depois da instalação, sem confiar exclusivamente no exit code.
- **Falha segura e auditável.** Eventos, relatórios, integridade SHA-256 e testes de regressão.
- **GUI desacoplada.** Personalização visual e motion design não alteram o núcleo.

## Ambientes Git
| Ambiente | Branch | Regra |
| --- | --- | --- |
| Desenvolvimento | `develop` | Implementação e integração, com validação no GitHub Actions |
| Produção | `main` | Somente mudanças aprovadas e verificadas por PR |

Os builds no GitHub Actions geram artefatos de desenvolvimento ou produção conforme a branch, **sem executar instaladores reais no runner**. O workflow usa os GitHub Environments `development` e `production`. As políticas de revisão/proteção desses environments exigem configuração administrativa adicional no GitHub.

## Fonte técnica
A arquitetura segue **Instalador Offline de Software — Documentação Técnica** (106 páginas, outubro de 2026): Go, catálogo JSON versionado, verificação SHA-256, `journal`, `kit-builder`, fases 0–5 e portões de qualidade.

## Limites atuais
O repositório começa sem binários nem uma coleção pré-carregada de instaladores. O catálogo será preenchido com payloads reais, legais, verificados e testados por pacote. **Não confundir manifesto de exemplo com software instalável.**

## Autoria
**Davi Brand — Software Developer**
