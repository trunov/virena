package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	Port           string `env:"PORT" envDefault:"8080"`
	DatabaseURI    string `env:"DATABASE_URI"`
	SendgridAPIKey string `env:"SENDGRID_API_KEY"`
}

func ReadConfig() (Config, error) {
	cfgEnv := Config{}

	if err := env.Parse(&cfgEnv); err != nil {
		return cfgEnv, err
	}

	cfgFlag := Config{}

	flag.StringVar(&cfgFlag.Port, "p", cfgEnv.Port, "port")
	flag.StringVar(&cfgFlag.DatabaseURI, "d", cfgEnv.DatabaseURI, "database URI")
	flag.StringVar(&cfgFlag.SendgridAPIKey, "s", cfgEnv.SendgridAPIKey, "sendgrid API key")

	flag.Parse()

	return cfgFlag, nil
}
