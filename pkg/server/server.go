package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func RunServer(config Config) error {
	mux := chi.NewMux()
	SetupRoutes(config, mux)
	return http.ListenAndServe(config.ListenAddress, mux)
}
