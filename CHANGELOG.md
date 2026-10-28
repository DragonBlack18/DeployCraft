# Changelog

## Não liberado — Endurecimento de retomada (2026-10-28)

- Toda execução registra SHA-256 canônico do conteúdo lógico do catálogo no journal.
- `resume` recusa catálogo alterado mesmo com o mesmo `catalogVersion`.
- `journal.ReadRun` recusa `run.started` malformado ou com metadados divergentes,
  resultado de pacote com estado inválido e eventos após conclusão terminal.
- Testes de ordem de mapas, adulteração de regras de instalação, replay de seleção
  e política de não executar um processo após mismatch.

**Nota:** esse fingerprint não é assinatura digital; a confiança de origem continua
dependendo de Ed25519 e, futuramente, Authenticode. Ainda não comprova reinício
físico em VM.


## Não liberado — Fase 3B (SFX em laboratório) — 2026-10-28

- `kit-builder build --mode sfx` gera EXE Windows único com arquivos não comprimidos, índice JSON e rodapé com digest SHA-256. Não sobrescreve versões existentes.
- `PackSource` acessa entradas por offset, com leitura em streaming, recusa de travessia, sobreposição e índices adulterados.
- Motor SFX carrega somente os JSONs de catálogo e assinatura em cache estável e valida Ed25519 antes de executar ações; os instaladores permanecem no EXE até serem copiados seletivamente para o staging.
- O mesmo motor suporta `PayloadSource` de pasta ou SFX, sem alterar os drivers MSI/EXE.
- Testes Linux/Windows cobrem corrupção, caminhos perigosos, integridade dos arquivos e inicialização real da CLI SFX no Windows.
- Nenhum payload de terceiros, certificado privado ou chave de assinatura foi incorporado ao repositório.

**Aviso:** o EXE SFX ainda NÃO recebe Authenticode. É um artefato **LAB**, sujeito a homologação com MSI/EXE reais, certificado de código e testes após reinicialização. A `main` não receberá a Fase 3B antes de aprovação dos gates.


## Não liberado — Fase 3A (kit-builder + assinatura LAB) — 2026-10-28

- Importação local de MSI/EXE com `kit-builder add` e `update`: catálogo JSON, versão, hash SHA-256, origem, switches e detecção explícitos.
- `report` mostra tamanhos; `verify` recalcula hashes e opcionalmente valida assinatura Ed25519 com a chave pública fornecida pelo operador.
- `keygen` gera Ed25519 fora do repositório (sem substituir arquivos de chaves existentes).
- `build --mode folder` constrói uma pasta nova e isolada, somente com os pacotes referenciados, assina o catálogo e compila o motor Windows com a chave pública fixada no executável.
- Motor valida assinatura antes de executar e recusa catálogo adulterado, assinatura ausente e chave diferente. O flag LAB não ignora uma chave fixada.
- Testes automatizados de alteração de catálogo, importação, atualização, hash inválido, keygen e montagem assinada/não assinada.
- Não foi incluído nenhum instalador real, chave privada, certificado ou payload no Git.

**Ainda não homologado:** SFX, Authenticode, VM Windows com instaladores corporativos reais e reinício automático RunOnce não foram realizados. A Fase 3A continua LAB; não promover a `main` como executável de produção.


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
