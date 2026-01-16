package service

import "go.opendefense.cloud/solar/internal/webhook/handlers"

type RepositoryEvent struct {
	Payload []byte
}

type WebhookConfig struct {
	Path   string
	Flavor handlers.WebhookFlavor
	Auth   string
}
