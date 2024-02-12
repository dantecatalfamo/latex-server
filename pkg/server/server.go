package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

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
	srv := http.Server{Addr: config.ListenAddress, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	<-stop

	log.Println("Received SIGINT, stopping...")
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	// Close the database before returning the error
	srvErr := srv.Shutdown(ctx)

	if err := config.database.conn.Close(); err != nil {
		return err
	}

	if srvErr != nil {
		return srvErr
	}

	return nil
}
