# Integridade da retomada — DeployCraft (Fase 3C LAB)

O planner continua recalculando o plano no `resume`, como exige a seção 10.4 da
documentação técnica. A segurança adicional evita retomar sob regras diferentes
daquelas aprovadas inicialmente, mesmo quando a string `catalogVersion` não muda.

O evento `run.started` guarda `catalogDigest` (SHA-256 do contrato lógico
normalizado de `catalog.Index` + todos os manifestos, com IDs ordenados).
Isso NÃO é uma assinatura criptográfica: o arquivo JSONL ainda deve ser
protegido por ACLs do Windows. O motor separado valida Ed25519 quando a chave
pública está fixada no executável.

Ao retomar, o motor verifica caminho do kit, versão, digest e os metadados
originais. Um `run.started` inserido posteriormente com outra seleção,
mesmo kit ou digest não é aceito. Estados estranhos em `step.finished` também
são recusados.

A segurança prioriza falhar antes de instalar. Jornals legados que não incluem
`catalogDigest` deixam de ser elegíveis para retomada automática: execute nova
instalação sob catálogo validado, com nova confirmação do operador, após
confirmar manualmente o estado do computador.

## Ainda pendente
A retomada por `RunOnce` após reboot real não foi homologada e segue
desativada. O journal não pode proteger contra um administrador local que
controle simultaneamente código e estado; a ameaça é mitigada pela assinatura
do catálogo e pelos direitos do Windows, não pelo fingerprint isolado.
