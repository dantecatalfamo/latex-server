package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RunServer starts a server using the given configuration and listens
func RunServer(config Config) error {
	mux := chi.NewMux()
	mux.Use(middleware.RequestID)
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Logger)
	SetupRoutes(config, mux)
	return http.ListenAndServe(config.ListenAddress, mux)
}
