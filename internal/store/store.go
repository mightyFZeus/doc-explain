package store

import (
	"context"
	"errors"

	"github.com/mightyfzeus/doc-explain/internal/models"
	"gorm.io/gorm"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type Storage struct {
	Users interface {
		CreateUser(ctx context.Context, user models.User) error
		UserExists(ctx context.Context, email string) (bool, error)
	}
}

func NewStorage(db *gorm.DB) Storage {
	return Storage{
		Users: &UserStore{db: db},
	}
}
