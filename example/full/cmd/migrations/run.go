package main

import (
	"context"
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/config"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	env := fluxgo.ParseEnv[config.Env](fluxgo.EnvOptions{LoadFromFile: fluxgo.Pointer(".env.development"), Validate: true})

	flux := fluxgo.
		New(fluxgo.FluxGoConfig{Name: "Migrations"}).
		AddApm(fluxgo.ApmOptions{CollectorURL: env.Apm.CollectorUrl, Exporter: env.Apm.Exporter}).
		AddDatabase(fluxgo.DatabaseOptions{Instances: []fluxgo.DatabaseConn{{Dsn: env.Database.Dsn}}})

	if err := flux.RunMigrations(ctx, fluxgo.DatabaseMigrationsOptions{Dir: "shared/database/migrations"}); err != nil {
		panic(err)
	}

	seeds := `
		INSERT INTO "user" (name) VALUES ('John Doe');
	`

	if err := flux.RunSeeds(ctx, seeds); err != nil {
		panic(err)
	}
}
