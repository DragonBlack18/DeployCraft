# DeployCraft

**DeployCraft** é o motor universal de implantação de software **offline** para Windows 10/11 x64, com catálogo JSON extensível. O código é Go; os instaladores de terceiros ficam fora do Git e do executável.

## Estado atual
- **Fase 0 (Fundamentos):** concluída com GitHub Actions em Windows e Linux.
- **Fase 1 (MVP):** validada em laboratório por GitHub Actions, *não homologada para produção*. Há detecção real Windows, drivers MSI/EXE, staging SHA-256, runner e journal.
- **Fase 2 (LAB):** perfis, dependências, conflitos, dry-run, execução em lote e retomada manual pelo runId. Reinício real de VM e RunOnce ainda pendentes.
- **Fase 3A (LAB):** importação e atualização de instaladores, montagem de kit em pasta e assinatura Ed25519 do catálogo com chave pública fixada no motor. **SFX e Authenticode ainda não implementados.**
- **Pendências obrigatórias:** validação E2E em máquina virtual limpa com instaladores reais, assinatura de catálogo, proteção do staging/controle de árvore, planejamento de dependências, retomada após reinício e interface desktop.

## Dois ambientes
| Ambiente | Branch | Política |
|---|---|---|
| Desenvolvimento | `develop` | Implementação e PR com testes automáticos |
| Produção | `main` | Somente funcionalidades verificadas e promovidas por PR |

Workflow: `.github/workflows/ci.yml` valida formatação, vet, build Windows, testes, catálogo e tamanho em Windows e Linux. Os artefatos ainda são **não assinados**; não executar em produção.

## Comandos de desenvolvimento
```powershell
git clone https://github.com/DragonBlack18/DeployCraft.git
cd DeployCraft
git switch develop
go run ./scripts/check
go run ./cmd/kit-builder lint --root .
go run ./cmd/kit-builder verify --root .
go run ./cmd/kit-builder report --root .
go run ./cmd/deploycraft list --kit .
go run ./cmd/deploycraft plan --kit . --profile desenvolvimento
```

## Teste de instalação em Windows VM somente
```powershell
go run ./cmd/deploycraft status --kit .
go run ./cmd/deploycraft install --kit . --dry-run --profile desenvolvimento
go run ./cmd/deploycraft install --kit . --lab-unsigned-catalog --dry-run <id>
go run ./cmd/deploycraft install --kit . --profile desenvolvimento --yes --lab-unsigned-catalog
go run ./cmd/deploycraft resume --kit . --yes --lab-unsigned-catalog <runId>
```

**Aviso:** requer PowerShell elevado, payload autêntico local, manifesto completo e `<id>` cadastrado. Os perfis começam vazios e o projeto não inclui nenhum instalador real. A exceção `--lab-unsigned-catalog` serve apenas a teste descartável, nunca a máquinas corporativas.

Para importar MSI/EXE, gerar chaves e montar um kit offline, siga `docs/FASE3-LAB.md`.

Documentação técnica complementar: `docs/ARCHITECTURE.md`, `docs/ENVIRONMENTS.md`, `docs/CATALOG.md`, `docs/ROADMAP.md` e `docs/FASE1-LAB.md`.

Autoria: **Davi Brand — Software Developer**.
