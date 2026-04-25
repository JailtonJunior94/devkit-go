# Prompt Enriquecido: Configuração de CI/CD e Release Automática

## Contexto do Repositório
- **Linguagem:** Go 1.26.2.
- **Arquitetura:** Monolito modular orientado a toolkit (`pkg/`).
- **Automação Local:** `Makefile` com comandos `lint`, `test`, `test-integration`, `vulncheck`.
- **Versionamento:** Conventional Commits e SemVer 2.0.0.
- **Documentação:** `CHANGELOG.md` centraliza o histórico de mudanças.

## Objetivo
Implementar o workflow `.github/workflows/ci.yml` que garanta a qualidade do código em cada PR e automatize a criação de tags e releases no GitHub ao realizar merge na branch principal.

## Bootstrap Operacional da Primeira Release
- A primeira publicação oficial deve ocorrer em uma branch controlada usando `workflow_dispatch`, antes da ativação operacional definitiva em `main`.
- Sem tags prévias, o bootstrap deve publicar exatamente `v0.1.0`, tratando o estado atual do repositório como baseline curado.
- O `CHANGELOG.md` deve conter exatamente uma seção `## [v0.1.0]` com corpo não vazio antes da execução do bootstrap.
- Após a validação bem-sucedida em branch controlada, o fluxo pode seguir o caminho normal de `push` para `main`.

## Instruções para o Agente de Implementação

### 1. Estrutura do Workflow (CI)
Crie jobs paralelos para as seguintes validações, utilizando o `Makefile` como fonte da verdade:
- **Lint:** Executar `make lint`. Recomenda-se o uso de `golangci/golangci-lint-action` para aproveitar o cache, garantindo o uso do `.golangci.yml`.
- **Unit Tests:** Executar `make test`. Garanta que o race detector esteja ativo (conforme definido no Makefile).
- **Integration Tests:** Executar `make test-integration`. Como o projeto utiliza `testcontainers-go`, assegure que o runner do GitHub Actions tenha suporte a Docker habilitado.
- **Vulnerability Check:** Executar `make vulncheck` ou utilizar `google/govulncheck-action`.

### 2. Automação de Release (CD)
Configure um job de release que dispare apenas após o sucesso dos jobs de CI na branch `main`:
- **Cálculo de Versão:** Analisar os commits desde a última tag seguindo o padrão Conventional Commits para determinar o próximo SemVer.
- **Tagging:** Criar e realizar o push da nova tag Git (ex: `v1.1.0`).
- **GitHub Release:**
    - Criar a release oficial no repositório.
    - **Extração de Changelog:** O conteúdo da release deve ser extraído do arquivo `CHANGELOG.md`. O script/action deve localizar a seção da nova versão e capturar apenas o conteúdo relevante (Markdown) para popular o corpo da release no GitHub.

### 2.1 Regras de Fail-Fast
- A automação deve falhar cedo quando não houver Conventional Commits elegíveis para incremento SemVer desde a última tag.
- A automação deve falhar cedo quando a seção esperada da versão estiver ausente, duplicada ou vazia no `CHANGELOG.md`.
- O comportamento de falha precisa ser tratado como governança operacional, não como ajuste manual posterior.

### 3. Restrições e Segurança
- Utilize `permissions` granulares (ex: `contents: write`, `pull-requests: read`).
- O workflow deve falhar se qualquer passo de CI falhar, impedindo a release.
- Evite redundância de lógica entre o `Makefile` e os arquivos de workflow.

## Critérios de Aceitação
- Arquivo `.github/workflows/ci.yml` funcional e validado.
- Separação clara entre jobs de validação e job de entrega.
- Suporte a cache de dependências Go para acelerar o pipeline.
- Mecanismo robusto de leitura do `CHANGELOG.md` para as notas de release.
- Execução inicial documentada via `workflow_dispatch` em branch controlada para o bootstrap de `v0.1.0`.
- Dependência explícita de Conventional Commits para cálculo de versão incremental após o bootstrap.

---
**Documentação de Referência:**
- Conventional Commits: https://www.conventionalcommits.org/
- Semantic Versioning: https://semver.org/
- Govulncheck: https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck
