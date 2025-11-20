package fluxgo

import (
	"context"
	"fmt"
	"log"

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
				err := db.Ping()
				if err != nil {
					return err
				}
				f.log("DATABASE", "Connected")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				err := db.Close()
				if err != nil {
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

func (d *Database) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, Span) {
	return d.apm.StartSpan(ctx, name, opts...)
}

type Entity interface {
	TableName() string
	PrimaryKey() string
}

type Repository[T Entity] struct {
	DB         *Database
	tableName  string
	primaryKey string
}

func NewRepository[T Entity](db *Database, tableName string) *Repository[T] {
	var entity T

	return &Repository[T]{
		DB:         db,
		tableName:  entity.TableName(),
		primaryKey: entity.PrimaryKey(),
	}
}

func (o *Repository[T]) StartSpan(ctx context.Context, opts ...trace.SpanStartOption) (context.Context, Span) {
	opts = append(opts, trace.WithAttributes(attribute.String("db.system", "postgresql")))
	return o.DB.StartSpan(ctx, fmt.Sprintf("repository/%s/%s", o.tableName, FunctionCaller(2)), opts...)
}
