package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"gorm.io/gorm"
)

type ConversationStore struct {
	db *gorm.DB
}

func (cs *ConversationStore) GetOrCreateByDocumentID(
	ctx context.Context,
	documentID uuid.UUID,
) (models.DocumentConversation, error) {
	var conversation models.DocumentConversation

	err := cs.db.WithContext(ctx).
		Where("document_id = ?", documentID).
		First(&conversation).Error

	if err == nil {
		return conversation, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return conversation, err
	}

	conversation = models.DocumentConversation{
		ID:         uuid.New(),
		DocumentID: documentID,
	}

	err = cs.db.WithContext(ctx).Create(&conversation).Error
	return conversation, err
}

func (cs *ConversationStore) GetByDocumentID(ctx context.Context, documentID uuid.UUID) ([]models.DocumentConversation, error) {
	var conversations []models.DocumentConversation

	err := cs.db.WithContext(ctx).
		Joins("JOIN documents ON documents.id = document_conversations.document_id").
		Where("document_conversations.document_id = ?", documentID).
		Where("documents.deleted_at IS NULL").
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Order("document_conversations.updated_at DESC").
		Find(&conversations).Error

	return conversations, err
}

func (cs *ConversationStore) CreateMessage(ctx context.Context, message models.DocumentMessage) error {
	if message.ID == uuid.Nil {
		message.ID = uuid.New()
	}

	return cs.db.WithContext(ctx).Create(&message).Error
}

func (cs *ConversationStore) GetRecentMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]models.DocumentMessage, error) {
	if limit <= 0 {
		limit = 6
	}

	var messages []models.DocumentMessage
	err := cs.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
