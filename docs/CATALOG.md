# Catálogo universal de softwares — contrato

`catalog/catalog.json` contém a versão, o índice de arquivos e os perfis. Cada instalador tem um `catalog/packages/<id>.json` e seus arquivos físicos locais em `payloads/<id>/`, nunca versionados no Git.

| Tecnologia | Driver | Estado |
| --- | --- | --- |
| MSI corporativo | `msi` | Manifesto validado; instalação fase 1 |
| EXE silencioso (Inno, NSIS, WiX etc.) | `exe` | Manifesto validado; instalação fase 1 |
| ZIP portátil | `portable` | Manifesto validado; instalação fase 4 |
| Scripts offline | `script` | Manifesto validado; instalação futura |
| APPX/MSIX | `msix` | Previsto fase 4; lint rejeita até a implementação |

"Universal" descreve extensibilidade por pacote e driver, **não suporte automático a todo instalador existente**. A origem oficial, direitos de redistribuição, modo silencioso e pós-detecção devem ser analisados individualmente.

## Grupos previstos
Essenciais, escritório/produtividade, navegadores, suporte e redes, desenvolvimento, drivers, runtime/dependências, programas industriais e aplicativos internos.

## Inclusão de pacote
1. Baixar binário completo offline pelo canal oficial e conferir licença de redistribuição.
2. Colocar o binário em `payloads/<id>/`, fora do Git.
3. Criar manifesto `catalog/packages/<id>.json` com versão, plataforma, caminho, hash e tamanho verdadeiros.
4. Registrar o arquivo no índice e adicionar aos perfis.
5. Rodar `go run ./cmd/kit-builder lint --root .` e `verify --root .`.
6. Em VM Windows, confirmar os switches, detecção antes e depois, segunda execução idempotente e remoção.
7. Atualizar CHANGELOG e gerar kit assinado (fase 3).

Os perfis iniciais estão vazios. **Não há binários reais, licenças embutidas ou instaladores de exemplo fingindo funcionar.**

## Formato
`schema/catalog.schema.json` e `schema/package.schema.json` definem o formato v1. O loader Go aplica validações de estrutura, referências e grafos. O comando `verify` verifica arquivos físicos de maneira independente.
