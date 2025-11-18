package main

import (
	"net/http"

	"github.com/trunov/virena/internal/app/config"
	"github.com/trunov/virena/internal/app/handler"
	"github.com/trunov/virena/internal/app/postgres"
	"github.com/trunov/virena/internal/app/services"
	"github.com/trunov/virena/logger"
)

func StartServer(cfg config.Config, dbStorage postgres.DBStorager) *http.Server {
	l := logger.Get()

	s := services.NewFileService()

	h := handler.NewHandler(dbStorage, s, l, cfg.SendgridAPIKey)
	r := handler.NewRouter(h)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	return srv
}
