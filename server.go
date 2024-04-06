package main

import (
	"net/http"

	"github.com/trunov/virena/internal/app/config"
	"github.com/trunov/virena/internal/app/handler"
	"github.com/trunov/virena/internal/app/postgres"
	"github.com/trunov/virena/internal/app/services"
	"github.com/trunov/virena/logger"
)

func StartServer(cfg config.Config, dbStorage postgres.DBStorager) {
	l := logger.Get()

	s := services.NewFileService()

	h := handler.NewHandler(dbStorage, s, l, cfg.SendgridAPIKey)
	r := handler.NewRouter(h)

	l.Info().
		Msgf("Starting the Virena app server on PORT '%s'", cfg.Port)

	l.Fatal().
		Err(http.ListenAndServe(":"+cfg.Port, r)).
		Msg("Virena App Server Closed")
}
