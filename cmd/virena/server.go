package main

import (
	"net/http"

	"github.com/trunov/virena/internal/app/config"
	"github.com/trunov/virena/internal/app/handler"
	"github.com/trunov/virena/internal/app/postgres"
	"github.com/trunov/virena/logger"
)

func StartServer(cfg config.Config, dbStorage postgres.DBStorager) {
	l := logger.Get()

	h := handler.NewHandler(dbStorage, l)
	r := handler.NewRouter(h)

	l.Info().
		Msgf("Starting the Virena app server on address '%s'", cfg.RunAddress)

	l.Fatal().
		Err(http.ListenAndServe(cfg.RunAddress+cfg.Port, r)).
		Msg("Virena App Server Closed")
}
