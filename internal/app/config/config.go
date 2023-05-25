package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	Port        string `env:"PORT" envDefault:"8080"`
	DatabaseURI string `env:"DATABASE_URI"`
}

func ReadConfig() (Config, error) {
	cfgEnv := Config{}

	if err := env.Parse(&cfgEnv); err != nil {
		return cfgEnv, err
	}

	cfgFlag := Config{}

	flag.StringVar(&cfgFlag.Port, "p", cfgEnv.Port, "port")
	flag.StringVar(&cfgFlag.DatabaseURI, "d", cfgEnv.DatabaseURI, "database URI")

	flag.Parse()

	return cfgFlag, nil
}
