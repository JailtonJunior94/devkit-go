package chiserver

import "github.com/go-chi/chi/v5"

// Router defines the contract for registering routes on the Chi router.
type Router interface {
	Register(router chi.Router)
}
