package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/trunov/virena/logger"

	"github.com/trunov/virena/internal/app/config"
	"github.com/trunov/virena/internal/app/repo"
)

func main() {
	l := logger.Get()
	ctx := context.Background()

	cfg, err := config.ReadConfig()
	if err != nil {
		l.Fatal().
			Err(err).
			Msgf("Failed to read the config.")
	}

	dbStorage, dbpool, err := repo.CreateRepo(ctx, cfg)
	if err != nil {
		l.Fatal().
			Err(err).
			Msgf("Error occurred while repository was initiating.")
	}
	defer dbpool.Close()

	srv := StartServer(cfg, dbStorage)

	idleConnsClosed := make(chan struct{})
	sigint := make(chan os.Signal, 1)

	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigint

		if err := srv.Shutdown(context.Background()); err != nil {
			l.Error().
				Err(err).
				Msg("HTTP server Shutdown")
		}

		close(idleConnsClosed)
	}()

	l.Info().Msgf("Starting the Virena app server on PORT '%s'", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		l.Fatal().
			Err(err).
			Msg("Virena App Server Closed unexpectedly")
	}

	<-idleConnsClosed

	l.Info().Msg("Server Shutdown gracefully")
}
