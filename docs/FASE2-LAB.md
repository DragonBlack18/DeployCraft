# Fase 2 — Planejamento LAB

Os contratos seguem as seções 6.5, 8, 10.4, 10.6 e 15.2 da documentação de 106 páginas.

## Comportamento
O `planner` é puro e faz expansão de perfis por `extends`, inclusão transitiva de `dependsOn`, rejeição de ciclos e conflitos com pacotes selecionados ou já instalados, ordenação topológica por Kahn com desempate por ID, idempotência, atualização e fases marcadas por `rebootBarrier`. Estado de detecção `desconhecido` bloqueia o plano, nunca vira `ausente`.

`plan` e `install --dry-run` não gravam no sistema. `install` exige `--yes --lab-unsigned-catalog`, Windows elevado e kit local, e verifica **todos os hashes** necessários antes do primeiro passo. Falha de dependência pula dependentes, mas não impede outros pacotes independentes, salvo `--stop-on-error`.

Após retorno de reinício (código 10) com fases restantes, `resume <runId>` lê `journal.jsonl`, recupera a seleção inicial, exige mesmo kit e mesma versão de catálogo, refaz a detecção e o plano. Um pacote concluído não é forçado na retomada mesmo que a primeira execução usasse `--force`.

## Comandos (flags antes dos IDs posicionais)

```powershell
DeployCraft.exe list --kit C:/DeployCraft-Kit
DeployCraft.exe plan --kit C:/DeployCraft-Kit --profile suporte-ti
DeployCraft.exe plan --kit C:/DeployCraft-Kit --profile desenvolvimento --json
DeployCraft.exe install --kit C:/DeployCraft-Kit --profile suporte-ti --dry-run

# SOMENTE VM Windows descartável, console elevado
DeployCraft.exe install --kit C:/DeployCraft-Kit --profile suporte-ti --yes --lab-unsigned-catalog
DeployCraft.exe install --kit C:/DeployCraft-Kit --yes --lab-unsigned-catalog --stop-on-error 7zip chrome

# Se houve saída 10 com fases restantes, reinicie e retome manualmente
DeployCraft.exe resume --kit C:/DeployCraft-Kit --yes --lab-unsigned-catalog <runId>
```

## Persistência
`%ProgramData%/DeployCraft/state/journal.jsonl` mantém eventos append-only e metadados `run.started`: seleção, opções, catalogVersion e kit. `state.json` é atualizado atomicamente e `logs/<runId>/report.json` registra resultados. Linha incompleta no fim do journal é ignorada; corrupção de linha completa impede retomada. Segredos nunca são persistidos.

## Gate de segurança ainda pendente
Não existe assinatura de catálogo nem Authenticode. O flag LAB não autoriza implantação corporativa. `RunOnce` automático **não está habilitado**, pois exigirá manifesto UAC e teste real de reboot em VM; só a retomada manual e simulada foi implementada. Instalação MSI/EXE de terceiros, reboot real, rede/pendrive após logon e publicação em produção precisam de homologação independente dos testes com fakes no Actions.
