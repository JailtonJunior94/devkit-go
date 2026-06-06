# DevKit Go

![devkit-go banner](https://raw.githubusercontent.com/JailtonJunior94/devkit-go/main/assets/banner.png)

[![Go Report Card](https://goreportcard.com/badge/github.com/JailtonJunior94/devkit-go)](https://goreportcard.com/report/github.com/JailtonJunior94/devkit-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg)](https://github.com/RichardLitt/standard-readme)

O **DevKit Go** é um ecossistema de bibliotecas e ferramentas de alta performance, projetado para acelerar o ciclo de desenvolvimento de microserviços em Go. Nosso foco é fornecer componentes prontos para produção com **Observabilidade**, **Segurança** e **Resiliência** nativos.

## Índice

- [Contexto](#contexto)
- [Instalação](#instalação)
- [Pacotes e Módulos](#pacotes-e-módulos)
    - [Core Engine](#core-engine)
    - [Infraestrutura e Dados](#infraestrutura-e-dados)
    - [Comunicação e APIs](#comunicação-e-apis)
    - [Utilitários e Segurança](#utilitários-e-segurança)
- [Princípios de Design](#princípios-de-design)
- [Governança e IA](#governança-e-ia)
- [Como Contribuir](#como-contribuir)
- [Licença](#licença)

## Contexto

Em um ecossistema de microserviços distribuídos, a padronização é a chave para a manutenibilidade. O DevKit Go resolve problemas comuns (boilerplate) de forma opinativa e eficiente, permitindo que as equipes foquem exclusivamente na lógica de negócio. Cada pacote foi desenhado para ser leve, seguindo as melhores práticas da comunidade Go (Effective Go).

## Instalação

Como o projeto é um monorepo de utilitários, você pode importar o pacote específico que necessita:

```bash
go get github.com/JailtonJunior94/devkit-go/pkg/<pacote>
```

## Pacotes e Módulos

### Core Engine

O alicerce de qualquer serviço construído com o DevKit.

- **[Observability](./pkg/observability/README.md)**: Telemetria unificada (Logs, Metrics, Tracing) baseada em OpenTelemetry.
- **[Logger](./pkg/logger/)**: Interfaces e implementações de logging estruturado.
- **[VOs (Value Objects)](./pkg/vos/)**: Tipos de domínio comuns como UUID, Email e CPFs com validação integrada.

### Infraestrutura e Dados

Acesso a dados robusto e transacional.

- **[Database](./pkg/database/README.md)**: Manager multi-driver (Postgres, MySQL, MSSQL) com Unit of Work genérico.
- **[Migration](./pkg/database/migration/)**: Motor de migração de banco de dados integrado ao startup.
- **[Messaging](./pkg/messaging/)**: Abstrações para mensageria assíncrona (Kafka, RabbitMQ).
- **[Worker](./pkg/worker/)**: Orquestra jobs agendados e consumers com shutdown gracioso e telemetria integrada.

### Comunicação e APIs

Padronização de interfaces de entrada e saída.

- **[HTTP Server](./pkg/http_server/README.md)**: Servidor unificado com adapters para **Chi** e **Fiber**.
- **[HTTP Client](./pkg/httpclient/)**: Cliente resiliente com telemetria e suporte a contextos.
- **[Responses](./pkg/responses/)**: Estruturas padronizadas para respostas de API (RFC 7807).

### Utilitários e Segurança

Ferramentas para o dia a dia do desenvolvedor.

- **[Encryption](./pkg/encrypt/)**: Utilitários para Hashing (BCrypt, Argon2) e criptografia AES.
- **[Linq](./pkg/linq/)**: Manipulação funcional de coleções (slices/maps) inspirada em C#.
- **[Nullable](./pkg/nullable/)**: Tipos que aceitam nulo com segurança para integrações de API e DB.

## Princípios de Design

1. **Performance em Primeiro Lugar**: Minimizamos alocações e evitamos reflexão desnecessária em caminhos críticos.
2. **Observabilidade Nativa**: Tudo o que o DevKit faz deixa um rastro (log, métrica ou trace).
3. **Contratos Claros**: Uso intensivo de interfaces e tipos fortes para evitar erros em tempo de execução.
4. **Segurança por Padrão**: Sanitização de erros, limites de buffers e TLS obrigatório onde necessário.

## Governança e IA

Este projeto utiliza ferramentas avançadas de automação e governança para agentes de IA.
- Veja nosso **[AGENTS.md](./AGENTS.md)** para entender como os agentes operam neste repositório.
- Instruções específicas para Claude/Gemini podem ser encontradas em **[CLAUDE.md](./CLAUDE.md)**.

## Como Contribuir

Adoramos contribuições! Para manter a qualidade, pedimos que:
1. Abra uma **Issue** para discutir mudanças maiores.
2. Certifique-se de que seu código possui **testes unitários**.
3. Siga o padrão de **Conventional Commits**.

## Licença

Este projeto está licenciado sob a licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes. (MIT © Jailton Junior)
