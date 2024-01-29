package server

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const ContextAuthedUserKey = "authedUser"

// TokenAuthMiddleware checks the request for a bearer token, and if
// that token matches a user in the database, it adds that user to the
// request context under the ContextAuthedUserKey key
func TokenAuthMiddleware(config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestId := middleware.GetReqID(r.Context())
			var authedUser string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				split := strings.Split(authHeader, " ")
				if len(split) > 1 && split[0] == "Bearer" {
					token := split[1]
					user, err := GetUserFromToken(config, token)
					if err != nil {
						log.Printf("[%s] TokenAuthMiddleware bad auth token \"%s\": %s", requestId, token, err)
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

// GetAuthedUser retrieves the authorized user set by TokenAuthMiddleware
func GetAuthedUser(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if user, ok := ctx.Value(ContextAuthedUserKey).(string); ok {
		return user
	}
	return ""
}

// IsUserAuthed checks if a given user is authorized against the
// header set by TokenAuthMiddleware
func IsUserAuthed(ctx context.Context, user string) bool {
	return GetAuthedUser(ctx) == user
}

// AuthProtectProjectMiddleware will allow a request to pass if a
// project is public, or if the user who created the project is
// authorized according to the ContextAuthedUserKey context key set by
// TokenAuthMiddleware
func AuthProtectProjectMiddleware(config Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := chi.URLParam(r, "user")
			project := chi.URLParam(r, "project")
			authedUser := GetAuthedUser(r.Context())
			requestId := middleware.GetReqID(r.Context())
			public, err := config.database.IsProjectPublic(user, project)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					http.Error(w, "404 page not found", http.StatusNotFound)
					return
				}
				log.Printf("[%s] AuthProtectProjectMiddleware: %s", requestId, err)
				http.Error(w, "internal service error", http.StatusInternalServerError)
				return
			}
			if public || user == authedUser {
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, "404 page not found", http.StatusNotFound)
			}
		})
	}
}
