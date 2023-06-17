package main

import (
	"context"
	"fmt"

	"github.com/trunov/virena/logger"

	"github.com/trunov/virena/internal/app/config"
	"github.com/trunov/virena/internal/app/file"
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

	fmt.Println(cfg)
	dbStorage, dbpool, err := repo.CreateRepo(ctx, cfg)
	if err != nil {
		l.Fatal().
			Err(err).
			Msgf("Error occurred while repository was initiating.")
	}
	defer dbpool.Close()

	err = file.SeedTheDB("skoda.csv", dbpool, ctx)
	if err != nil {
		l.Fatal().
			Err(err).
			Msgf("Error occurred while repository was initiating.")
	}

	StartServer(cfg, dbStorage)
}
