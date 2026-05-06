# DevKit Go

O DevKit Go é uma coleção de bibliotecas Go de alta performance e reutilizáveis, projetadas para acelerar o desenvolvimento de microserviços com foco em observabilidade, segurança e produtividade do desenvolvedor.

## Pacotes

Este repositório contém as seguintes bibliotecas:

### Bibliotecas Principais

- **[Observabilidade](./pkg/observability/README.md)**: Logs estruturados, métricas e rastreamento unificados com OpenTelemetry.
- **[Database](./pkg/database/)**: Gerenciamento de conexões multi-driver (Postgres, CockroachDB, MySQL, MSSQL), Unit of Work genérico e migração de banco de dados.
- **[HTTP Server](./pkg/httpserver/)**: Implementação padronizada de servidor HTTP.
- **[HTTP Client](./pkg/httpclient/)**: Cliente HTTP ciente de contexto com telemetria integrada.
- **[Messaging](./pkg/messaging/)**: Abstrações para corretores de mensagens (Kafka, RabbitMQ).
- **[Encryption](./pkg/encrypt/)**: Utilitários de segurança para hashing e criptografia.

### Utilitários

- **[Linq](./pkg/linq/)**: Consultas integradas à linguagem (LINQ) para slices e maps em Go.
- **[Nullable](./pkg/nullable/)**: Tipos anuláveis (nullable) com segurança de tipo para compatibilidade com banco de dados e APIs.
- **[Logger](./pkg/logger/)**: Interfaces de log padronizadas.
- **[Migration](./pkg/database/migration/)**: Ferramentas para migração de banco de dados (parte de `pkg/database`).
- **[Responses](./pkg/responses/)**: Estruturas padronizadas para respostas de API.
- **[VOs](./pkg/vos/)**: Objetos de Valor (Value Objects) comuns (Email, UUID, etc.).

## Como Começar

Cada pacote contém sua própria documentação. Recomendamos começar pelo pacote de **[Observabilidade](./pkg/observability/README.md)** para estabelecer a base do seu serviço.

## Princípios

- **Performance em Primeiro Lugar**: Alocações mínimas e estruturas de dados eficientes.
- **Observabilidade por Design**: Cada pacote é construído para ser observável.
- **Simplicidade**: APIs claras, fáceis de testar e manter.

## Licença

[Adicionar Informações de Licença Aqui]
