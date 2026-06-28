package documentworker

import (
	"context"

	"github.com/mightyfzeus/doc-explain/internal/models"
)

func (p *DocumentProcessor) trackAnalytics(ctx context.Context, event models.AnalyticsEvent) {
	if err := p.store.Analytics.Track(ctx, event); err != nil {
		p.logger.Errorw("failed to track analytics event", "eventType", event.EventType, "error", err)
	}
}
