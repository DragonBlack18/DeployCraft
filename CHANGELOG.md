# Changelog

## Não liberado — Fase 2 (Planejamento LAB) — 2026-10-28
- Planner puro: perfis herdados, dependências transitivas, Kahn determinístico, conflito em ambos os sentidos, estados instalados/desatualizados/desconhecidos e barreiras de reboot.
- CLI `plan`, `install --dry-run`, `install --profile`, IDs múltiplos e `resume <runId>`.
- Batch verifica hash de todos os pacotes necessários antes de executar o primeiro, continua independentes após falha e bloqueia os dependentes.
- Journal contém seleção original, opções, catálogo e caminho do kit; retomar replaneja e respeita idempotência.
- Testes golden, DAG aleatório, conflitos, falha por dependência, hash e retomada.
- Retomada **manual** até haver RunOnce homologado após reinicialização de VM e assinatura de catálogo.


## Não liberado — Fase 1 / LAB (2026-10-28)

- Detector real Windows com registro HKLM (64/32), HKCU e regras de arquivo.
- Driver MSI/EXE com argumentos sem shell e mascaramento de segredos no log.
- Staging local: cópia verificada por SHA-256 e tamanho, recusa de symlinks externos.
- Runner com timeout, captura limitada de saída e código de término.
- Registro JSON Lines persistente, snapshot atômico e relatório de execução.
- CLI status/install com bloqueio explícito de catálogo não assinado e exigência de --yes.
- Testes em Windows e Linux com adaptadores falsos e APIs reais de leitura do Windows.
- Instalações reais de softwares de terceiros ainda precisam de VM homologada.
- Detecção de estado desconhecido bloqueia execução; exit code 0 sozinho não prova sucesso.


## Não liberado — Fase 0 (2026-10-28)

- DeployCraft adotado como nome definitivo.
- Branches `develop` e `main` para desenvolvimento e produção.
- Catálogo versionado; perfis iniciais vazios, sem payloads fictícios.
- Motor Go com contratos, comparação numérica de versões e loader rígido.
- `kit-builder lint`, `kit-builder verify` (SHA-256) e quality gate.
- GitHub Actions: validação em Windows/Linux e artefatos separados por environment.

**Não existe release operacional.** Instalação real, detecção, journal, retomar, GUI e assinatura são etapas futuras.
