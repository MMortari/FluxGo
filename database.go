package fluxgo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/multierr"
)

type databaseData struct {
	primaryDBs []*sqlx.DB
	replicaDBs []*sqlx.DB
}

type Database struct {
	apm  *Apm
	once sync.Once
	mu   sync.Mutex

	dbs           map[string]*databaseData
	mainPrimaryDB *sqlx.DB
	mainReplicaDB *sqlx.DB
}
type DatabaseConn struct {
	Dsn  string
	Type string
}
type DatabaseOptions struct {
	Name      string
	Instances []DatabaseConn
}

func (f *FluxGo) AddDatabase(data DatabaseOptions) *FluxGo {
	f.db.mu.Lock()
	defer f.db.mu.Unlock()

	name := "default"
	if data.Name != "" {
		name = data.Name
	}

	if _, exists := f.db.dbs[name]; exists {
		log.Fatalf("Database with name %s already exists", name)
	}

	database := databaseData{
		primaryDBs: []*sqlx.DB{},
		replicaDBs: []*sqlx.DB{},
	}

	for _, item := range data.Instances {
		db, err := sqlx.Open("postgres", item.Dsn)
		if err != nil {
			log.Fatalln("Error to create database client", err)
		}

		switch item.Type {
		case "replica":
			database.replicaDBs = append(database.replicaDBs, db)
		default:
			database.primaryDBs = append(database.primaryDBs, db)
		}
	}

	f.db.dbs[name] = &database

	f.db.once.Do(func() {
		f.AddInvoke(func(lc fx.Lifecycle, db *Database) error {
			f.db.apm = f.apm

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if err := db.Connect(ctx); err != nil {
						return err
					}
					f.log("DATABASE", "Connected")
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := db.Disconnect(); err != nil {
						return err
					}
					f.log("DATABASE", "Disconnected")
					return nil
				},
			})
			return nil
		})
	})

	return f
}

func (d *Database) Connect(ctx context.Context) error {
	var err error

	for _, dbs := range d.dbs {
		for _, db := range dbs.primaryDBs {
			if e := db.PingContext(ctx); e != nil {
				err = multierr.Append(err, e)
			}
		}
		for _, db := range dbs.replicaDBs {
			if e := db.PingContext(ctx); e != nil {
				err = multierr.Append(err, e)
			}
		}
	}

	return err
}
func (d *Database) Disconnect() error {
	var err error

	for _, dbs := range d.dbs {
		for _, db := range dbs.primaryDBs {
			if e := db.Close(); e != nil {
				err = multierr.Append(err, e)
			}
		}
		for _, db := range dbs.replicaDBs {
			if e := db.Close(); e != nil {
				err = multierr.Append(err, e)
			}
		}
	}

	return err
}

func (d *Database) WriteDBNamed(name string) *sqlx.DB {
	db, exists := d.dbs[name]
	if !exists {
		panic(fmt.Sprintf("No database configured with name %s", name))
	}

	switch size := len(db.primaryDBs); size {
	case 0:
		panic("No primary database configured")
	case 1:
		return db.primaryDBs[0]
	default:
		index := GetRandomNumber(size)
		return db.primaryDBs[index]
	}
}
func (d *Database) WriteDB() *sqlx.DB {
	if d.mainPrimaryDB != nil {
		return d.mainPrimaryDB
	}

	var key string

	switch size := len(d.dbs); size {
	case 0:
		panic("No database configured")
	case 1:
		for k := range d.dbs {
			key = k
		}
	default:
		panic("You must use WriteDBNamed when more than one database is configured")
	}

	db := d.WriteDBNamed(key)

	d.mainPrimaryDB = db

	return db
}
func (d *Database) ReadOnlyDBNamed(name string) *sqlx.DB {
	db, exists := d.dbs[name]
	if !exists {
		panic(fmt.Sprintf("No database configured with name %s", name))
	}

	switch size := len(db.replicaDBs); size {
	case 0:
		return d.WriteDBNamed(name)
	case 1:
		return db.replicaDBs[0]
	default:
		index := GetRandomNumber(size)
		return db.replicaDBs[index]
	}
}
func (d *Database) ReadOnlyDB() *sqlx.DB {
	if d.mainReplicaDB != nil {
		return d.mainReplicaDB
	}

	var key string

	switch size := len(d.dbs); size {
	case 0:
		panic("No database configured")
	case 1:
		for k := range d.dbs {
			key = k
		}
	default:
		panic("You must use ReadOnlyDBNamed when more than one database is configured")
	}

	db := d.ReadOnlyDBNamed(key)

	d.mainReplicaDB = db

	return db
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

				driver, err := postgres.WithInstance(db.WriteDB().DB, postgresConfig)
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

				if _, err := db.WriteDB().ExecContext(ctx, query); err != nil {
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
