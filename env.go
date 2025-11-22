package fluxgo

import (
	"log"

	"github.com/caarlos0/env/v11"
)

type Env[T any] struct {
	Data T
}

func ParseEnv[T any]() T {
	config, err := env.ParseAs[T]()
	if err != nil {
		log.Fatal("Error to parse env:", err)
	}
	return config
}
