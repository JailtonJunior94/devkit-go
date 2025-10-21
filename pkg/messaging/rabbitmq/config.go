package rabbitmq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	DeadLetterSuffix = "dlq"
	RetrySuffix      = "retry"
)

type AmqpBuilder struct {
	channel       *amqp.Channel
	exchanges     []*Exchange
	bindings      []*Binding
	ttl           time.Duration
	withRetry     bool
	withDLQ       bool
	prefetchCount int
}

func NewAmqpBuilder(ch *amqp.Channel) *AmqpBuilder {
	return &AmqpBuilder{
		channel:       ch,
		withRetry:     false,
		withDLQ:       false,
		ttl:           10 * time.Second,
		prefetchCount: 2,
	}
}

func (a *AmqpBuilder) DeclareExchanges(exchanges ...*Exchange) *AmqpBuilder {
	a.exchanges = exchanges
	return a
}

func (a *AmqpBuilder) DeclareBindings(bindings ...*Binding) *AmqpBuilder {
	a.bindings = bindings
	return a
}

func (a *AmqpBuilder) DeclareTTL(ttl time.Duration) *AmqpBuilder {
	a.ttl = ttl
	return a
}

func (a *AmqpBuilder) DeclarePrefetchCount(prefetchCount int) *AmqpBuilder {
	a.prefetchCount = prefetchCount
	return a
}

func (a *AmqpBuilder) WithDLQ() *AmqpBuilder {
	a.withDLQ = true
	return a
}

func (a *AmqpBuilder) WithRetry() *AmqpBuilder {
	a.withRetry = true
	return a
}

func (a *AmqpBuilder) Apply() (*AmqpBuilder, error) {
	if err := a.channel.Qos(a.prefetchCount, 0, false); err != nil {
		return a, fmt.Errorf("amqp_builder: qos: %v", err)
	}

	if len(a.exchanges) > 0 {
		for _, exchange := range a.exchanges {
			if err := a.channel.ExchangeDeclare(exchange.Exchange, exchange.Kind, true, false, false, false, nil); err != nil {
				return a, fmt.Errorf("amqp_builder: exchange_declare: %v", err)
			}
		}
	}

	if len(a.bindings) > 0 {
		for _, binding := range a.bindings {
			_, err := a.channel.QueueDeclare(binding.Queue, true, false, false, false, a.queueArgs(binding.Queue))
			if err != nil {
				return a, fmt.Errorf("amqp_builder: queue_declare: %v", err)
			}

			if err := a.declareDLQ(binding.Queue); err != nil {
				return a, fmt.Errorf("amqp_builder: queue_declare_dlq: %v", err)
			}

			if err := a.declareRetry(binding.Queue); err != nil {
				return a, fmt.Errorf("amqp_builder: queue_declare_retry: %v", err)
			}

			if binding.Routing != nil {
				if err := a.channel.QueueBind(binding.Queue, *binding.Routing, binding.Exchange, false, nil); err != nil {
					return a, fmt.Errorf("amqp_builder: queue_bind: %v", err)
				}
			}

			if err := a.channel.QueueBind(binding.Queue, "", binding.Exchange, false, nil); err != nil {
				return a, fmt.Errorf("amqp_builder: queue_bind: %v", err)
			}
		}
	}

	return a, nil
}

func (a *AmqpBuilder) declareDLQ(queue string) error {
	if !a.withDLQ {
		return nil
	}

	dlqQueue := fmt.Sprintf("%s.%s", queue, DeadLetterSuffix)
	_, err := a.channel.QueueDeclare(dlqQueue, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("queue_declare: %s", err)
	}
	return nil
}

func (a *AmqpBuilder) declareRetry(queue string) error {
	if !a.withRetry {
		return nil
	}

	retryQueue := fmt.Sprintf("%s.%s", queue, RetrySuffix)
	_, err := a.channel.QueueDeclare(retryQueue, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": queue,
		"x-message-ttl":             a.ttl.Milliseconds(),
	})
	if err != nil {
		return fmt.Errorf("queue_declare: %s", err)
	}
	return nil
}

func (a *AmqpBuilder) queueArgs(queue string) amqp.Table {
	args := amqp.Table{}
	if a.withRetry {
		args["x-dead-letter-exchange"] = ""
		args["x-dead-letter-routing-key"] = fmt.Sprintf("%s.%s", queue, RetrySuffix)
	}
	return args
}

type Binding struct {
	Queue    string
	Exchange string
	Routing  *string
}

func NewBinding(queue, exchange string) *Binding {
	return &Binding{
		Queue:    queue,
		Exchange: exchange,
	}
}

func NewBindingRouting(queue, exchange, routing string) *Binding {
	return &Binding{
		Queue:    queue,
		Exchange: exchange,
		Routing:  &routing,
	}
}

type Exchange struct {
	Exchange string
	Kind     string
}

func NewExchange(exchange, kind string) *Exchange {
	return &Exchange{
		Exchange: exchange,
		Kind:     kind,
	}
}
