package config

import fluxgo "github.com/MMortari/FluxGo"

type Env struct {
	fluxgo.Env
	Database struct {
		Dsn string `env:"DATABASE_DSN" validate:"required"`
	}
	Redis struct {
		Addr string `env:"REDIS_ADDR" validate:"required,hostname_port"`
	}
	Kafka struct {
		Brokers string `env:"KAFKA_BROKERS" validate:"required"`
		SSL     string `env:"KAFKA_SSL" validate:"required,boolean"`
	}
	Logger struct {
		Type     string `env:"LOGGER_TYPE" validate:"required"`
		Level    string `env:"LOGGER_LEVEL" validate:"required"`
		FilePath string `env:"LOGGER_FILE_PATH" validate:"required"`
	}
	Apm struct {
		Exporter     string `env:"APM_EXPORTER" validate:"required,oneof=grpc console"`
		CollectorUrl string `env:"APM_COLLECTOR_URL" validate:"required,hostname_port"`
	}
}
