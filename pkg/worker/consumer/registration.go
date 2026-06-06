package consumer

type Registration struct {
	Name      string
	EventType string
	Handler   Handler
}
