package service

import (
	"context"
	"log"
	"time"
)

type WebhookService struct {
	events chan RepositoryEvent
}

func New() WebhookService {
	return WebhookService{
		events: make(chan RepositoryEvent),
	}
}

func (s WebhookService) Channel() <-chan RepositoryEvent {
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
			log.Println("Received event", event.Payload)
		}
	}
}
