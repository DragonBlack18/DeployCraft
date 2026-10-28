# DeployCraft — Fase 3A LAB: kit-builder e integridade offline

Este módulo segue as seções 11, 12, 14 e 15 da documentação técnica original de 106 páginas.

## O que está implementado

`kit-builder add/update` importa EXE/MSI locais em `payloads/<id>/`, versionando nome do arquivo, SHA-256, tamanho, instruções silenciosas e regras de detecção. Nenhuma chamada à internet ou download automático é necessário.

`kit-builder keygen` cria um par Ed25519 com chave privada obrigatoriamente **fora do repositório**. A chave pública não é secreta. Não são sobrescritos arquivos de chave existentes. O diretório das chaves deve existir e ter permissões de acesso restritas.

`kit-builder build --mode folder` cria uma **nova** pasta de kit; não altera nem apaga versões anteriores e não inclui payloads órfãos. Valida hashes da origem e da cópia, assina o catálogo no kit final e compila `DeployCraft.exe` com a chave pública fixada pelo `-ldflags`. A chave privada jamais é copiada para o kit.

A verificação de assinatura é feita **pelo próprio motor antes de executar**. Uma chave pública presente no catálogo não é aceita como âncora de confiança. Se o motor possuir chave fixa, `--lab-unsigned-catalog` não permite ignorar uma assinatura incorreta.

## Comandos sugeridos no PowerShell

Requer Go 1.24+ no computador de desenvolvimento. Execute na pasta do repositório, branch `develop` ou feature.

```powershell
go run ./scripts/check

# Criar diretório de chaves FORA do projeto, numa pasta privada do usuário
$KeysDir = Join-Path $env:USERPROFILE "DeployCraft-Keys"
New-Item -ItemType Directory -Path $KeysDir -Force | Out-Null

go run ./cmd/kit-builder keygen --root . `
  --private "$KeysDir/catalog.private.pem" `
  --public "$KeysDir/catalog.public.pem"

# EXEMPLO: usar um arquivo baixado legalmente do fornecedor, NÃO um placeholder.
go run ./cmd/kit-builder add --root . `
  --file "C:/Instaladores/7z-real-offline.exe" `
  --id 7zip --name "7-Zip" --version 24.08 `
  --driver exe --arg /S --detect-name "7-Zip*" `
  --profile essencial --source-url "https://www.7-zip.org/"

go run ./cmd/kit-builder lint --root .
go run ./cmd/kit-builder verify --root .
go run ./cmd/kit-builder report --root .

# Criar kit assinado por catálogo: NOVA pasta que ainda não existe
go run ./cmd/kit-builder build --root . --mode folder `
  --out dist/kit-2026.10.1 `
  --private-key "$KeysDir/catalog.private.pem"

# Conferir a cópia e a assinatura offline
go run ./cmd/kit-builder verify --root dist/kit-2026.10.1 `
  --public-key "$KeysDir/catalog.public.pem"
```

*O nome e caminho do instalador acima são exemplificativos; não são arquivos fornecidos pelo DeployCraft.* Você precisa do instalador real e dos switches validados pelo fabricante. Se a detecção não corresponder à instalação real, o pós-check impede falso sucesso.

## Modo de desenvolvimento não assinado

```powershell
go run ./cmd/kit-builder build --root . --mode folder `
  --out dist/kit-lab-001 --lab-unsigned
```

Este modo é exclusivamente de laboratório. Kits assinados Ed25519 **também não são releases corporativas** enquanto não houver assinatura Authenticode do motor e homologação de MSI/EXE reais em VM isolada.

## Não suportado nesta etapa

- `build --mode sfx`: indisponível; o comando deve falhar explicitamente (não há pacote fake).
- Assinatura Authenticode/certificado de código da empresa e carimbo RFC 3161.
- Importação automática de qualquer tecnologia desconhecida, ZIP portátil/script/MSIX.
- Confirmação automatizada de licenças de redistribuição ou legitimidade do fornecedor.
- Reinicialização real com RunOnce, sem VM de validação.

Nenhum desses itens pode ser descrito como pronto antes de testes específicos. A promoção à `main` continua bloqueada para novas funcionalidades LAB.
