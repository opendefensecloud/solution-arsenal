package service

import (
	"context"
	"log"
	"time"

	"go.opendefense.cloud/solar/pkg/discovery"
)

type WebhookService struct {
	events chan discovery.RepositoryEvent
}

func New() WebhookService {
	return WebhookService{
		events: make(chan discovery.RepositoryEvent),
	}
}

func (s WebhookService) Channel() chan<- discovery.RepositoryEvent {
	return s.events
}

func (s WebhookService) Start(ctx context.Context) error {
	time.NewTicker(time.Second * 1)
	for {
		select {
		case <-ctx.Done():
			log.Println("webhook service is shutting down")
			return ctx.Err()
		case event := <-s.events:
			log.Println("Received event", event.Type, event.Timestamp.Format(time.RFC3339))
		}
	}
}
