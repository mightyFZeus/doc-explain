package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/mightyfzeus/doc-explain/cmd/helpers"
	"github.com/mightyfzeus/doc-explain/internal/dtos"
	"github.com/mightyfzeus/doc-explain/internal/models"
)

func (app *application) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var payload dtos.UserDto
	ctx := r.Context()

	if err := helpers.DecodeAndValidate(w, r, &payload); err != nil {
		app.logger.Errorf("can't decode and validate: %v", err)
		return
	}
	if payload.Password != payload.ConfirmPassword {
		app.badRequestResponse(w, r, errors.New("password and confirm password must be the same"))
		return
	}

	ok := helpers.IsValidPasswordPCRE(payload.Password)
	if !ok {
		app.badRequestResponse(w, r, errors.New("password format not valid"))
		return
	}

	hashedPassword, err := helpers.HashPassword(payload.Password)
	if err != nil {
		app.logger.Errorf("unable to hash password: %v", err)
		app.badRequestResponse(w, r, err)
		return
	}

	user := models.User{
		Id:            uuid.New(),
		FullName:      payload.FullName,
		Email:         payload.Email,
		Password:      hashedPassword,
		TermsAccepted: payload.TermsAccepted,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := app.store.Users.CreateUser(ctx, user); err != nil {
		app.logger.Errorf("unable to create user to database: %v", err)
		app.badRequestResponse(w, r, err)
		return
	}

	app.jsonResponse(w, http.StatusCreated, user)

}

func (app *application) LogingHandler(w http.ResponseWriter, r *http.Request) {
	var payload dtos.LoginDto
	if err := helpers.DecodeAndValidate(w, r, &payload); err != nil {
		app.logger.Errorf("can't decode and validate: %v", err)
		return
	}

	ctx := r.Context()

	user, err := app.store.Users.LoginUser(ctx, payload.Email, payload.Password)
	if err != nil {
		app.logger.Errorf("unable to login user: %v", err)
		app.badRequestResponse(w, r, err)
		return
	}

	token, err := helpers.GenerateJWT(user.Id.String(), user.Email)
	if err != nil {
		app.logger.Errorf("unable to generate jwt: %v", err)
		app.badRequestResponse(w, r, err)
		return
	}

	response := dtos.LoginResponse{
		User:  user,
		Token: token,
	}

	app.jsonResponse(w, http.StatusOK, response)

}
