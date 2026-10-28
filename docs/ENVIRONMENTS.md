# Ambientes development e production

## Política Git

| Ambiente | Branch | Objetivo |
| --- | --- | --- |
| development | `develop` | Código em desenvolvimento, revisão e testes |
| production | `main` | Código aprovado, nunca implementação não validada |

Trabalhar com branches de feature derivadas de `develop`; integrar via pull requests. Promover `develop` para `main` somente depois dos quality gates e revisão. Sem force push, reset de branch compartilhada ou commits com instaladores pesados.

## GitHub Actions

O workflow `.github/workflows/ci.yml` executa a matriz `quality-ubuntu-latest` e `quality-windows-latest` em cada push ou PR. Cada job roda `go run ./scripts/check`, com formatação, go vet, build nativo e cruzado, testes, lint, integridade (quando payloads locais existem) e tamanho.

Após qualidade verde em **push**, o job `bundle` compila as duas CLIs Windows x64 e cria SHA256SUMS.txt. Os artefatos são **não assinados** e **não são releases instaláveis**.

O job seleciona o GitHub Environment `development` ao publicar da `develop`, ou `production` ao publicar da `main`. O GitHub cria o environment na primeira execução que o referenciar, caso ainda não exista.

**A criação automática não protege environments nem branches.**

## Ações administrativas necessárias

Em `Settings > Environments > production`, configure restrição à branch `main` e aprovadores quando a funcionalidade estiver disponível no plano do repositório. Em `Settings > Rules > Rulesets` (ou `Branches`), exija PR, bloqueie force push e exija os checks `quality-ubuntu-latest` e `quality-windows-latest` para `main`. Opcionalmente aplique regras parecidas à `develop`.

Essas restrições **não** são criadas pelo arquivo YAML; devem ser verificadas no painel de administração.

## Operação local (fase 0)

```powershell
git clone https://github.com/DragonBlack18/DeployCraft.git
cd DeployCraft
git switch develop
go run ./scripts/check
go run ./cmd/kit-builder lint --root .
go run ./cmd/deploycraft list --kit .
```

Desenvolvimento: Go 1.24+. O produto final não terá runtime nem internet como pré-requisitos nas máquinas de destino.

## Gate de release

CI verde é necessário, mas não suficiente para produção corporativa. Antes do primeiro release operacional serão exigidos instalação real em VM limpa, segunda rodada idempotente, validação de desinstalação, reinício e resume, integridade do kit, assinatura do catálogo e Authenticode do executável.
