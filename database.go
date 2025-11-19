package fluxgo

import (
	"context"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/fx"
)

type Database struct {
	*sqlx.DB
}
type DatabaseOptions struct {
	Dsn string
}

func (f *FluxGo) AddDatabase(opt DatabaseOptions) *FluxGo {
	f.AddDependency(func(lc fx.Lifecycle) *Database {
		db, err := sqlx.Connect("postgres", opt.Dsn)
		if err != nil {
			log.Fatalln("Error to connect on database", err)
		}

		database := Database{db}

		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return db.Ping()
			},
			OnStop: func(ctx context.Context) error {
				return db.Close()
			},
		})

		return &database
	})

	return f
}
