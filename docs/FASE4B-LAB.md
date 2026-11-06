# Fase 4B LAB — Windows Job Objects

## Escopo desta alteração

Base: `develop` no commit `015d6c321121091da3e47ee757740eb5f5b78b90`.
O runner anterior encerrava somente o processo inicial. A Fase 4B substitui
apenas o adaptador Windows; os contratos `domain.Process`, `domain.Running`,
os drivers e o catálogo não mudam. Os PRs de Authenticode, resume e retry
são independentes e não são declarados aprovados por este trabalho.

## Comportamento Windows 10/11 x64

- Um Job Object privado, sem nome e sem herança de handle é criado por tentativa.
- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` é configurado antes de iniciar o EXE.
- `CreateProcessW` usa `PROC_THREAD_ATTRIBUTE_JOB_LIST`, atribuindo o processo
  ao job na criação; também inicia suspenso e só então libera a thread.
  Não existe fallback para executar fora do job quando uma API falha.
- `PROC_THREAD_ATTRIBUTE_HANDLE_LIST` limita a herança a stdin e stdout/stderr.
  O handle do job nunca é entregue ao instalador. Breakaway não é habilitado.
- `Wait` observa `ActiveProcesses`, e não somente o processo principal.
  Um launcher que termina enquanto um filho trabalha não produz conclusão
  prematura. O código final preservado é o do processo principal; a confirmação
  funcional continua sendo a pós-detecção do motor.
- Timeout, cancelamento e `Kill` encerram o grupo associado por
  `TerminateJobObject`. A morte do processo controlador fecha o handle do job.
- No Windows, chamadas concorrentes ou repetidas de `Wait` reutilizam o mesmo
  resultado; os handles nativos têm um único responsável por liberação.
- Saída capturada mantém os últimos 16 KiB. Ambiente é Unicode e as substituições
  de variáveis respeitam nomes sem distinção entre maiúsculas/minúscculas.
- O backend não Windows conserva o comportamento anterior de processo direto.

## Testes adicionados

O próprio executável de testes cria processos auxiliares descartáveis. Não há
instalador de fornecedor, certificado, modificação do Registro ou UAC nos testes.

| Caso | Evidência exigida |
| --- | --- |
| Deadline, cancelamento e Kill | Handles de pai, filho e neto ficam sinalizados |
| Launcher sai antes do filho | Wait retorna somente após marcador de conclusão do filho |
| Queda abrupta do controlador | Os descendentes terminam via kill-on-close |
| Wait concorrente/repetido | Código 17 é preservado sem reutilizar handles liberados |
| Argumentos, Unicode, ambiente e diretório | Comparação literal no processo criado |
| Falha ao criar job | Nenhum marcador do payload é produzido |
| Buffer concorrente e escrita de 1 MiB | Saída retida não ultrapassa 16 KiB |

O workflow `Windows Job Objects` repete os testes nativos três vezes, sem cache,
com limite de tempo e publicação do log. O workflow de qualidade existente
continua responsável pela regressão completa Windows/Linux e pelo race detector
Linux. Um teste compilado para Windows **não equivale** a executá-lo no Windows.

## Limites e homologação

Job Objects não são um sandbox de segurança. A contenção abrange processos
associados ao job e filhos herdados; não garante cancelar trabalho delegado a
serviços externos, Windows Installer Service, WMI ou outros brokers. Não se deve
usar `taskkill` global nem matar serviços compartilhados para compensar isso.

Instaladores que exigem breakaway podem falhar; não se remove a proteção
silenciosamente. A política atual aguarda toda a árvore: um instalador que abre
um aplicativo persistente deve ter essa opção desativada em seus switches.

Não há rollback automático nesta etapa. A finalização de uma árvore não desfaz
arquivos/Registro alterados. MSI/EXE de fornecedores, processos elevados,
serviços, reboot real, staging com ACLs e Authenticode corporativo continuam
pendentes de homologação. Este código permanece LAB até os gates aplicáveis.

## Referências técnicas consultadas

- Microsoft Learn — Job Objects: https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects
- Microsoft Learn — UpdateProcThreadAttribute (JOB_LIST e HANDLE_LIST): https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute
- Microsoft — criação diretamente no job, sem janela de associação: https://devblogs.microsoft.com/oldnewthing/20230209-00/?p=107812

Nenhum certificado privado, payload de terceiros ou binário de testes é
versionado. O código de validação local e seu go.mod temporário não fazem parte
do commit do produto.
