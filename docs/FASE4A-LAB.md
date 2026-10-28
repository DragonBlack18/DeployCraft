# Fase 4A LAB — Tentativas transitórias com espera crescente

Os códigos definidos em `install.retryCodes` são os **únicos** elegíveis a
nova tentativa, conforme a seção 10.1 da documentação técnica do DeployCraft.

- Uma execução inicial e no máximo **três** novas tentativas.
- Esperas progressivas de 15, 30 e 60 segundos.
- A espera é cancelável via `context.Context`; nenhuma thread fica dormindo
  sem capacidade de resposta ao cancelamento.
- A cada tentativa, um novo processo é criado, sem shell intermediário.
- As tentativas são registradas com `step.retry` no journal.
- Entre tentativas, o detector real é consultado novamente. Se o aplicativo
  já foi instalado por outro processo, não o reinstalamos.
- Estado de detecção `desconhecido` bloqueia novas tentativas.
- Timeout, falha ao iniciar processo e integridade inválida **nunca** são
  repetidos, pois efeitos colaterais poderiam ser desconhecidos.
- Código persistente 1618 após a terceira espera retorna E-RUN-003, não
  relata falso sucesso nem descarta a última saída.
- Códigos diferentes de `retryCodes` não disparam retries.

## Gates
Os testes automatizados utilizam executores falsos com códigos [1618, 0],
[1618, 1618, 1618, 1618] e [1603, 0], injeção de relógio/espera para
evitar lentidão e cancelamento. Regressão completa em Windows e Linux.

**Limites:** Job Objects, árvore de processos e retomada após reboot real
ainda não estão homologados. Retries de instaladores MSI corporativos
devem ser testados em VM antes de qualquer release operacional.
