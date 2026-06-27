package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mightyfzeus/doc-explain/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserStore struct {
	db *gorm.DB
}

func (us *UserStore) CreateUser(ctx context.Context, user models.User) error {
	err := us.db.WithContext(ctx).Create(&user).Error
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailAlreadyExists
		}
		return err
	}

	return nil
}

func (us *UserStore) UserExists(ctx context.Context, email string) (bool, error) {
	var count int64

	err := us.db.WithContext(ctx).
		Model(&models.User{}).
		Where("email = ?", email).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (us *UserStore) UserExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	var count int64

	err := us.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (u *UserStore) LoginUser(ctx context.Context, email, password string) (*models.User, error) {
	var user models.User
	err := u.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		switch {
		case err.Error() == "record not found":
			return nil, ErrUserNotFound
		default:
			return nil, err
		}

	}

	if !checkPassword(password, user.Password) {
		return nil, errors.New("invalid credentials")
	}

	return &user, nil

}

func checkPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
