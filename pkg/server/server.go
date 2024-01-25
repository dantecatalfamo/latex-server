package server

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// RunServer starts a server using the given configuration and listens
func RunServer(config Config) error {
	mux := chi.NewMux()
	mux.Use(middleware.RealIP)
	mux.Use(middleware.Logger)
	mux.Use(TokenAuthMiddleware(config))
	SetupRoutes(config, mux)
	return http.ListenAndServe(config.ListenAddress, mux)
}

const ContextAuthedUserKey = "authedUser"

func TokenAuthMiddleware(config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var authedUser string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				split := strings.Split(authHeader, " ")
				if len(split) > 1 && split[0] == "Bearer" {
					token := split[1]
					user, err := GetUserFromToken(config, token)
					if err != nil {
						log.Printf("Bad auth token \"%s\": %s", token, err)
					} else {
						authedUser = user
					}
				}
			}
			ctx := context.WithValue(r.Context(), ContextAuthedUserKey, authedUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
