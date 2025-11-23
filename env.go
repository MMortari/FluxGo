package fluxgo

import (
	"log"
	"os"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

type EnvOptions struct {
	LoadFromFile *string
	Validate     bool
}

func ParseEnv[T any](opts EnvOptions) T {
	var empty T

	if opts.LoadFromFile != nil {
		if _, err := os.Stat(*opts.LoadFromFile); err != nil {
			return empty
		}

		err := godotenv.Load(*opts.LoadFromFile)
		if err != nil {
			log.Printf("Error loading %s file: %v", *opts.LoadFromFile, err)
		}
	}

	config, err := env.ParseAs[T]()
	if err != nil {
		log.Fatal("Error to parse env:", err)
	}

	if opts.Validate {
		validate := validator.New()
		if errs := validate.Struct(config); errs != nil {
			log.Fatal("Invalid environment ", errs)
		}
	}

	return config
}

type Env struct {
	Env string `env:"ENV"`
}

func (e Env) IsProduction() bool {
	return e.Env == "production"
}
func (e Env) IsSandbox() bool {
	return e.Env == "sandbox"
}
func (e Env) IsDevelopment() bool {
	return e.Env == "development"
}
func (e Env) IsTest() bool {
	return e.Env == "test"
}

// IsDeployed returns true if the environment is either production or sandbox.
// It is used to determine if the application is running in a deployed state.
func (e Env) IsDeployed() bool {
	return e.IsProduction() || e.IsSandbox()
}
