package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	Id            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey;not null" json:"id" validate:"required"`
	FullName      string    `gorm:"type:varchar(255);not null" json:"fullName" validate:"required"`
	Email         string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"email" validate:"required"`
	Password      string    `gorm:"type:varchar(255);not null" json:"-" validate:"required"`
	TermsAccepted bool      `gorm:"type:boolean;not null" json:"-" validate:"required"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
