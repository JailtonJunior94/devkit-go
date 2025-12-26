package responses

import (
	"encoding/json"
	"log"
	"net/http"
)

// JSON escreve uma resposta JSON no ResponseWriter.
// Se houver erro ao codificar, retorna HTTP 500 com mensagem de erro.
// É thread-safe e não causa panic, adequado para uso em produção.
func JSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log o erro mas não faz panic - não derruba a aplicação
		log.Printf("error encoding JSON response: %v", err)

		// Tenta enviar resposta de erro (se ainda não enviamos o body)
		// Nota: se já escrevemos parte do body, isso pode não funcionar
		// mas pelo menos não derrubamos a aplicação
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	}
}

// Error escreve uma resposta de erro JSON com uma mensagem.
// Adequado para erros simples sem detalhes adicionais.
func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, struct {
		Message string `json:"message"`
	}{
		Message: message,
	})
}

// ErrorWithDetails escreve uma resposta de erro JSON com mensagem e detalhes adicionais.
// Útil para validação de entrada e erros que requerem contexto adicional.
func ErrorWithDetails(w http.ResponseWriter, statusCode int, message string, details any) {
	JSON(w, statusCode, struct {
		Message string `json:"message"`
		Details any    `json:"details,omitempty"`
	}{
		Message: message,
		Details: details,
	})
}
