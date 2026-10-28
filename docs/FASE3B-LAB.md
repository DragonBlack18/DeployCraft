# Fase 3B LAB — SFX offline de arquivo único

## Contrato da documentação
A estrutura segue a seção 11.4 da documentação técnica original, sem ZIP nem extração em massa:

```text
[DeployCraft.exe][entradas não compactadas][índice JSON][rodapé]
```

O rodapé contém o magic `DCPACK01`, offset e tamanho do índice e hash SHA-256 do índice. Cada entrada possui caminho relativo, offset, tamanho e SHA-256. O leitor recusa duplicatas, percursos perigosos, índices adulterados, sobreposição de offsets e dados fora do limite. A leitura por `PackSource.Open` retorna somente a faixa de bytes solicitada.

Se o executável já tiver Authenticode, o leitor considera a tabela PE Security para procurar o rodapé antes do certificado, descontando até sete bytes de alinhamento. **Essa compatibilidade ainda precisa de teste com um certificado real.** A integração com SignTool está disponível para desenvolvimento na Fase 3C. A assinatura e a homologação com um certificado corporativo ainda não foram realizadas.

## Criação do arquivo único
O kit-builder continua utilizando somente o acervo local. O resultado gerado fica em `dist/` (fora do Git), nunca é sobrescrito e não inclui a chave privada. Se o catálogo estiver numa pasta separada do código Go, informe também `--source-root <caminho-do-repositório>` para compilar o motor sem procurar `go.mod` na pasta de payloads.

```powershell
# Laboratório: manifesto sem assinatura (somente testes isolados)
go run ./cmd/kit-builder build --mode sfx --root . --out dist/DeployCraft-LAB.exe --lab-unsigned

# Catálogo assinado (ainda sem Authenticode do executável)
go run ./cmd/kit-builder build --mode sfx --root . --out dist/DeployCraft-SFX.exe --private-key "$env:USERPROFILE/DeployCraft-Keys/catalog.private.pem"

# Leitura somente do catálogo embutido
& .\dist\DeployCraft-SFX.exe list
```

`list` carrega apenas os JSONs do catálogo. O motor armazena os metadados em cache, numa pasta estável por digest do índice, para permitir o mesmo identificador de kit na retomada manual. Não descompacta antecipadamente todos os instaladores.

Em `install`, o planejador confere os hashes dos pacotes selecionados antes da execução. O driver copia somente o arquivo selecionado para staging verificado e realiza pós-detecção. `resume` permanece manual e requer o mesmo SFX e a mesma conta do Windows.

## O que não está homologado
- Assinatura Authenticode do executável final e carimbo RFC 3161.
- Teste com SFX Authenticode real, tabela PE Security e Windows Defender/SmartScreen.
- Reinício real de VM, RunOnce/UAC e restauração após queda de energia.
- Instaladores MSI/EXE de fornecedores reais e licença de redistribuição.
- Não use `--lab-unsigned-catalog` fora de VM controlada.

O GitHub Actions executa apenas testes seguros com binários próprios de teste, sem instalar software de terceiros. Até esses gates serem aprovados, **SFX é um recurso de desenvolvimento**, não uma release corporativa.
