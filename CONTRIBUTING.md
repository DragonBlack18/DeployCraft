# Colaborando com DeployCraft

1. Criar features a partir de `develop`. Alterar `main` apenas após PR e validação.
2. Fazer alterações pequenas, relacionadas e documentadas no CHANGELOG.
3. Executar `go run ./scripts/check` antes de criar PR.
4. Para bugs: teste reproduzindo a falha antes da correção.
5. Nunca versionar `payloads/`, `dist/`, certificados, segredos ou executáveis.
6. Nunca executar testes reais de instalação fora de VM isolada sem autorização.
7. Não realizar force push ou reset destrutivo.
8. Não declarar implementação concluída sem evidência do gate correspondente.
