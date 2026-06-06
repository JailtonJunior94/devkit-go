---
name: feedback-no-underscore-prefix
description: Usuário não usa prefixo _ para constantes/variáveis não exportadas em Go — use camelCase simples (ex: defaultTimeout, não _defaultTimeout)
metadata:
  type: feedback
---

Não usar prefixo `_` para globais não exportados (constantes, variáveis de pacote).

**Why:** O usuário explicitamente rejeitou `_defaultShutdownTimeout`, afirmando "golang não usa _ underline". A preferência local prevalece sobre a regra 5.26 do Uber Go Style Guide.

**How to apply:** Sempre usar camelCase simples para constantes e variáveis não exportadas: `defaultShutdownTimeout`, `defaultTimeout`, `maxRetries`, etc. Nunca `_defaultShutdownTimeout`.
