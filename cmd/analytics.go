package main

import (
	"context"

	"github.com/mightyfzeus/doc-explain/internal/models"
)

func (app *application) trackAnalytics(ctx context.Context, event models.AnalyticsEvent) {
	if err := app.store.Analytics.Track(ctx, event); err != nil {
		app.logger.Errorw("failed to track analytics event", "eventType", event.EventType, "error", err)
	}
}
