# Segurança do DeployCraft

O instalador final terá privilégios elevados; por isso, o padrão é falhar antes de qualquer execução incerta.

- Nunca executar payload sem hash válido e procedência verificada.
- Nenhum executável de terceiros, segredo ou certificado privado no Git.
- Nunca colocar credenciais em JSON, comandos registrados ou logs.
- Nenhum download ou telemetria nas máquinas de destino.
- Scripts e executáveis serão permitidos somente de um kit homologado.
- Não construir comandos via shell arbitrário; evitar injeção e caminhos inseguros.
- Isolamento de staging, assinatura de catálogo e Authenticode serão requisitos antes de produção operacional.
- Não desabilitar antivírus para "resolver" bloqueio.
- Testes de instalação real serão feitos em máquina virtual descartável.

Esta fase valida catálogos e integridade, mas **não é um instalador homologado**.

Relate vulnerabilidades sensíveis ao mantenedor por canal privado, sem publicar segredos ou binários internos em issues.
