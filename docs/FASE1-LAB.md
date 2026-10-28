# DeployCraft — Fase 1: instalação em laboratório

## O que funciona
- `DeployCraft status --kit <kit>`: lê as regras reais de registro e arquivo em Windows.
- `DeployCraft install --kit <kit> --lab-unsigned-catalog --dry-run <id>`: visualiza o passo isolado sem executar nada.
- `DeployCraft install --kit <kit> --yes --lab-unsigned-catalog <id>`: **apenas em VM Windows descartável** com privilégios elevados.
- MSI/EXE com argumentos em lista, verificação de tamanho e SHA-256 em cópia local, timeout, pós-detecção e registro JSONL.
- A segunda instalação pula quando a versão adequada já está detectada.
- `go run ./scripts/check`: qualidade Linux/Windows e build cruzado.

## Restrições deliberadas
- O catálogo ainda não é assinado. A opção `--lab-unsigned-catalog` é uma exceção explícita **exclusiva para laboratório**, não deve ser usada em estações da empresa.
- Somente um id por execução. Dependências e conflitos são recusados até a Fase 2.
- `portable`, `script`, `msix` e `rawCmdLine` ainda não são executados.
- A detecção por `command`, `appx` e `file.minVersion` retorna `desconhecido`, bloqueando a instalação em vez de supor ausência.
- Timeouts ainda encerram o processo inicial, não a árvore inteira. Job Objects chegam na Fase 4.
- Reinício é reportado, mas retomada automática entra na Fase 2.
- Testes automatizados usam instaladores falsos; o aceite de **instaladores reais em VM limpa** permanece necessário antes da homologação.

## Kit local esperado
```text
kit/
  DeployCraft.exe
  catalog/
    catalog.json
    packages/
      <id>.json
  payloads/
    <id>/
      instalador-offline.msi (ou .exe)
```

O repositório contém código e JSON, mas nenhum payload de terceiros. Os perfis do índice começam vazios. Binários não devem entrar no Git.

## Auditoria
O motor escreve somente na máquina de teste, em `%ProgramData%\DeployCraft\state\journal.jsonl`, `state\state.json`, `logs\<runId>\report.json` e `staging\`. Os payloads copiados ao staging são apagados após a tentativa. O motor não escreve na pasta do kit.

## Próximo gate
Testar o instalador MSI e EXE reais em Windows Sandbox/VM com snapshot, confirmar instalação e segunda execução, testar mídia somente leitura, reinício e ausência de privilégios. Essas evidências não podem ser substituídas por mocks nem por um job do GitHub Actions.
