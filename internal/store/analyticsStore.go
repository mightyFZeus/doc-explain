package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AnalyticsStore struct {
	db *gorm.DB
}

func (as *AnalyticsStore) Track(ctx context.Context, event models.AnalyticsEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	if event.Count == 0 {
		event.Count = 1
	}

	if event.ActorType == "" {
		event.ActorType = models.AccountTypeRegistered
	}

	return as.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "dedupe_key"}},
			DoNothing: true,
		}).
		Create(&event).Error
}

func (as *AnalyticsStore) CountByUser(ctx context.Context, userID uuid.UUID, eventType string) (int64, error) {
	var count int64

	err := as.db.WithContext(ctx).
		Model(&models.AnalyticsEvent{}).
		Where("user_id = ?", userID).
		Where("event_type = ?", eventType).
		Select("COALESCE(SUM(count), 0)").
		Scan(&count).Error

	return count, err
}
