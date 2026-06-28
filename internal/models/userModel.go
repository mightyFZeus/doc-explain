package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	AccountTypeRegistered = "registered"
	AccountTypeGuest      = "guest"
)

func ActorTypeForAccount(accountType string) string {
	if accountType == AccountTypeGuest {
		return AccountTypeGuest
	}

	return AccountTypeRegistered
}

type User struct {
	Id             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey;not null" json:"id" validate:"required"`
	FullName       string     `gorm:"type:varchar(255);not null" json:"fullName" validate:"required"`
	Email          string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"email" validate:"required"`
	Password       string     `gorm:"type:varchar(255);not null" json:"-" validate:"required"`
	TermsAccepted  bool       `gorm:"type:boolean;not null" json:"-" validate:"required"`
	AccountType    string     `gorm:"type:varchar(30);not null;default:'registered';index" json:"accountType"`
	GuestExpiresAt *time.Time `json:"guestExpiresAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
