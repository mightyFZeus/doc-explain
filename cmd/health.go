package main

import "net/http"

func (app *application) HealthHandler(w http.ResponseWriter, r *http.Request) {
	type healthResponse struct {
		Status string `json:"status"`
		Env    string `json:"env"`
	}

	response := healthResponse{
		Status: "ok",
		Env:    app.config.env,
	}

	writeJSON(w, http.StatusOK, response)
}
