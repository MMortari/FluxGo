package fluxgo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

type Database struct {
	*sqlx.DB
	apm *Apm
}
type DatabaseOptions struct {
	Dsn string
}

func (f *FluxGo) AddDatabase(opt DatabaseOptions) *FluxGo {
	f.AddDependency(func(apm *Apm) *Database {
		db, err := sqlx.Open("postgres", opt.Dsn)
		if err != nil {
			log.Fatalln("Error to create database client", err)
		}

		database := Database{db, apm}

		return &database
	})
	f.AddInvoke(func(lc fx.Lifecycle, db *Database) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := db.Ping(); err != nil {
					return err
				}
				f.log("DATABASE", "Connected")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := db.Close(); err != nil {
					return err
				}
				f.log("DATABASE", "Disconnected")
				return nil
			},
		})
		return nil
	})

	return f
}

type DatabaseMigrationsOptions struct {
	Dir    string
	Config *postgres.Config
}

func (f *FluxGo) RunMigrations(ctx context.Context, opt DatabaseMigrationsOptions) error {
	opts := append(f.GetFxConfig(), fx.Invoke(func(lc fx.Lifecycle, db *Database) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				log.Println("Starting migrations...")

				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("error to get pwd: %v", err)
				}

				joined := path.Join(wd, opt.Dir)
				migrationsDirFile := "file://" + joined

				var postgresConfig = opt.Config
				if postgresConfig == nil {
					postgresConfig = &postgres.Config{}
				}

				driver, err := postgres.WithInstance(db.DB.DB, postgresConfig)
				if err != nil {
					return fmt.Errorf("error to get pg driver: %v", err)
				}

				migrator, err := migrate.NewWithDatabaseInstance(migrationsDirFile, "postgres", driver)
				if err != nil {
					return fmt.Errorf("unable to create migration: %v", err)
				}
				defer func() {
					if err1, err2 := migrator.Close(); err1 != nil || err2 != nil {
						log.Printf("error closing migrator: %v", err1)
						log.Printf("error closing migrator: %v", err2)
					}
				}()

				if err := migrator.Up(); err != nil {
					if errors.Is(err, migrate.ErrNoChange) {
						log.Println("No changes to be applied")
						return nil
					} else {
						return fmt.Errorf("unable to apply migrations %v", err)
					}
				}

				log.Println("Migrations done!")

				return nil
			},
		})
	}), fx.NopLogger)

	return fx.New(opts...).Start(ctx)
}
func (f *FluxGo) RunSeeds(ctx context.Context, query string) error {
	opts := append(f.GetFxConfig(), fx.Invoke(func(lc fx.Lifecycle, db *Database) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				log.Println("Starting seeds...")

				if _, err := db.ExecContext(ctx, query); err != nil {
					return fmt.Errorf("unable to apply seeds: %v", err)
				}

				log.Println("Seeds done!")

				return nil
			},
		})
	}), fx.NopLogger)

	return fx.New(opts...).Start(ctx)
}

func (d *Database) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, Span) {
	return d.apm.StartSpan(ctx, name, opts...)
}

type Entity interface {
	TableName() string
	PrimaryKey() string
}

type Repository[T Entity] struct {
	DB         *Database
	TableName  string
	PrimaryKey string
}

func NewRepository[T Entity](db *Database) *Repository[T] {
	var entity T

	return &Repository[T]{
		DB:         db,
		TableName:  entity.TableName(),
		PrimaryKey: entity.PrimaryKey(),
	}
}

func (o *Repository[T]) StartSpan(ctx context.Context, opts ...trace.SpanStartOption) (context.Context, Span) {
	opts = append(opts, trace.WithAttributes(attribute.String("db.system", "postgresql")))
	return o.DB.StartSpan(ctx, fmt.Sprintf("repository/%s/%s", o.TableName, FunctionCaller(2)), opts...)
}
