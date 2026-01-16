package handlers

import "net/http"

type WebhookHTTPHandler interface {
	http.Handler
}

type WebhookFlavor string

const (
	WebhookFlavorZot WebhookFlavor = "zot"
)
