// cmd/documentStatusEvents.go
package main

import (
	"context"
	"encoding/json"

	"github.com/mightyfzeus/doc-explain/internal/dtos"
)

func (app *application) ListenForDocumentStatusEvents(ctx context.Context) {
	pubsub := app.redis.Subscribe(ctx, dtos.DocumentStatusChannel)
	defer pubsub.Close()

	for msg := range pubsub.Channel() {
		var event dtos.DocumentStatusEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			app.logger.Errorw("invalid document status event", "error", err)
			continue
		}

		app.docHub.Broadcast(event)
	}
}
