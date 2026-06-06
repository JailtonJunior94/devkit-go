package consumer

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
)

type consumerRunner struct {
	name        string
	source      Source
	reg         *registry
	obs         observability.Observability
	dispCounter observability.Counter
	errCounter  observability.Counter
}

func NewRunner(name string, source Source, registrations []Registration, obs observability.Observability) (Runner, error) {
	reg := newRegistry()
	for _, r := range registrations {
		if err := reg.register(r.EventType, r.Handler); err != nil {
			return nil, fmt.Errorf("worker: consumer %s register %s: %w", name, r.EventType, err)
		}
	}

	return &consumerRunner{
		name:        name,
		source:      source,
		reg:         reg,
		obs:         obs,
		dispCounter: obs.Metrics().Counter("worker.consumers.dispatches_total", "Total consumer dispatches", "1"),
		errCounter:  obs.Metrics().Counter("worker.consumers.errors_total", "Total consumer dispatch errors", "1"),
	}, nil
}

func (r *consumerRunner) Start(ctx context.Context) error {
	messages, err := r.source.Messages(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			r.handleMessage(ctx, msg)
		}
	}
}

func (r *consumerRunner) Stop(ctx context.Context) error {
	return r.source.Stop(ctx)
}

func (r *consumerRunner) handleMessage(ctx context.Context, msg Message) {
	ctx, span := r.obs.Tracer().Start(ctx, "worker.consumer.dispatch",
		observability.WithAttributes(
			observability.String("consumer.name", r.name),
			observability.String("event.type", msg.EventType),
		))
	defer span.End()

	if err := r.reg.dispatch(ctx, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(observability.StatusCodeError, err.Error())
		r.errCounter.Increment(ctx,
			observability.String("consumer", r.name),
			observability.String("event_type", msg.EventType),
		)
		r.obs.Logger().Error(ctx, "consumer dispatch failed",
			observability.String("operation", "worker.consumer.dispatch"),
			observability.String("name", r.name),
			observability.String("event_type", msg.EventType),
			observability.Error(err),
		)
		return
	}

	r.dispCounter.Increment(ctx,
		observability.String("consumer", r.name),
		observability.String("event_type", msg.EventType),
		observability.String("result", "success"),
	)
}
