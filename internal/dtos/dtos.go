package dtos

import "github.com/google/uuid"

type UserDto struct {
	FullName        string `gorm:"not null" json:"fullName" validate:"required,min=2,max=255"`
	Email           string `gorm:"not null" json:"email" validate:"required,min=2,max=255"`
	Password        string `gorm:"not null" json:"password" validate:"required,min=2,max=72"`
	ConfirmPassword string `gorm:"not null" json:"confirmPassword" validate:"required"`
	TermsAccepted   bool   `gorm:"type:boolean;not null" json:"termsAccepted" validate:"required,eq=true"`
}

type SearchDocumentDto struct {
	Query      string    ` json:"query" validate:"required"`
	DocumentID uuid.UUID ` json:"documentId" validate:"required"`
}
