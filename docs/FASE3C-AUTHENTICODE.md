# Fase 3C — Authenticode do SFX (LAB até homologação)

A documentação técnica de referência determina que o arquivo SFX seja montado
**antes** da assinatura do executável (seções 11.4, 12.5 e 14.3). O
DeployCraft agora possui um estágio separado de assinatura por SignTool,
compatível com PowerShell 5.1.

## 1. Certificado

É necessário um certificado de **assinatura de código emitido para o editor**,
disponível com chave privada utilizável em `Cert:\\CurrentUser\\My` ou
`Cert:\\LocalMachine\\My`. O projeto não gera certificados públicos,
não distribui PFX e não armazena senhas no código, YAML ou GitHub.

O certificado temporário autoassinado gerado no GitHub Actions tem assunto
`CN=DeployCraft CI Test`. Ele serve exclusivamente para demonstrar que o
SFX aceita uma assinatura PE e que a adulteração do motor é detectada; **não**
é uma credencial de produção e não cria reputação SmartScreen.

## 2. Fluxo de assinatura

Com um catálogo Ed25519 válido e payloads localmente autorizados:

```powershell
go run ./cmd/kit-builder build --root . --mode sfx `
    --out .\dist\DeployCraft-unsigned.exe `
    --private-key "C:\\LocalSeguro\\deploycraft-ed25519-private.pem"

go run ./cmd/kit-builder inspect-sfx --file .\dist\DeployCraft-unsigned.exe

# Substituir por thumbprint do certificado corporativo e URL TSA de sua organização
.\scripts\sign\Sign-DeployCraft.ps1 `
    -InputFile .\dist\DeployCraft-unsigned.exe `
    -OutputFile .\dist\DeployCraft-signed.exe `
    -Thumbprint "<THUMBPRINT_DO_CERTIFICADO>" `
    -TimestampUrl "<URL_RFC3161_DO_FORNECEDOR>"

.\scripts\sign\Test-Authenticode.ps1 `
    -File .\dist\DeployCraft-signed.exe `
    -ExpectedThumbprint "<THUMBPRINT_DO_CERTIFICADO>" `
    -RequireTimestamp

go run ./cmd/kit-builder inspect-sfx --file .\dist\DeployCraft-signed.exe
.\dist\DeployCraft-signed.exe list
```

Requer Windows SDK / `signtool.exe` e certificado corporativo válido.
O script localiza o SDK quando `SignToolPath` não foi informado; usa
`/fd SHA256 /tr <TSA> /td SHA256`. O timestamp é necessário fora do modo
CI, e falhar na validação impede a publicação. Os executáveis de entrada e
saída são **arquivos distintos** e o processo nunca sobrescreve a origem
ou o destino. Pode ser informado `-ExpectedSHA256` para fixar a origem
que passou por revisão.

## 3. O que cada teste demonstra

- `Get-AuthenticodeSignature` / `signtool verify /pa`: identidade do
  editor e confiança da cadeia de certificados naquela máquina.
- `kit-builder inspect-sfx`: hash do índice e de cada payload dentro
  do SFX; **não** verifica a assinatura Authenticode nem a confiança do editor.
- `DeployCraft.exe list`: leitura do catálogo embutido com chave pública
  Ed25519 compilada no motor, sem executar instaladores.
- Teste do GitHub Actions: certificado temporário na VM Windows,
  assinatura pós-montagem, leitura do SFX e rejeição após adulteração
  de uma região do motor.

**Defesa em profundidade:** Authenticode não é hash plano de todos os
bytes do executável PE (há regiões excluídas do cálculo). Por isso a
assinatura Ed25519 do catálogo e os hashes SHA-256 dos payloads continuam
obrigatórios. A integridade do índice SFX sozinha não confere origem.

## 4. Pendências reais

O usuário ainda não forneceu certificado de assinatura de código nem
URL TSA. Sem eles, não existe release Authenticode corporativa nem
timestamp real. Continuam pendentes instalação de MSI/EXE de terceiros,
reinicialização real com UAC/RunOnce, validação de reputação
SmartScreen e direitos de redistribuição.

O GitHub Actions **não** guarda chaves corporativas e não publica
o SFX autoassinado temporário como release.
