package fluxgo

import (
	"context"
	"testing"

	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

// TestEntity implementa a interface Entity para testes
type TestEntity struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func (t TestEntity) TableName() string {
	return "test_entities"
}

func (t TestEntity) PrimaryKey() string {
	return "id"
}

func TestDatabase_AddDatabase(t *testing.T) {
	t.Run("Should add primary database", func(t *testing.T) {
		fluxgo := New(FluxGoConfig{Name: "Test"})
		
		result := fluxgo.AddDatabase(DatabaseOptions{
			Dsn:  "postgres://test:test@localhost/test?sslmode=disable",
			Type: "primary",
		})
		
		assert.NotNil(t, result)
		assert.Equal(t, fluxgo, result)
	})

	t.Run("Should add replica database", func(t *testing.T) {
		fluxgo := New(FluxGoConfig{Name: "Test"})
		
		result := fluxgo.AddDatabase(DatabaseOptions{
			Dsn:  "postgres://test:test@localhost/test?sslmode=disable",
			Type: "replica",
		})
		
		assert.NotNil(t, result)
		assert.Equal(t, fluxgo, result)
	})

	t.Run("Should add multiple databases", func(t *testing.T) {
		fluxgo := New(FluxGoConfig{Name: "Test"})
		
		result := fluxgo.AddDatabase(
			DatabaseOptions{
				Dsn:  "postgres://test1:test@localhost/test1?sslmode=disable",
				Type: "primary",
			},
			DatabaseOptions{
				Dsn:  "postgres://test2:test@localhost/test2?sslmode=disable",
				Type: "replica",
			},
		)
		
		assert.NotNil(t, result)
		assert.Equal(t, fluxgo, result)
	})
}

func TestDatabase_Connect(t *testing.T) {
	t.Run("Should connect successfully with no databases", func(t *testing.T) {
		db := &Database{
			primaryDBs: []*sqlx.DB{},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		ctx := context.Background()
		err := db.Connect(ctx)
		
		assert.NoError(t, err)
	})

	t.Run("Should return error when database connection is invalid", func(t *testing.T) {
		// Cria um banco com DSN inválido para testar erro de conexão
		invalidDB, _ := sqlx.Open("postgres", "invalid://connection/string")
		
		db := &Database{
			primaryDBs: []*sqlx.DB{invalidDB},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		ctx := context.Background()
		err := db.Connect(ctx)
		
		assert.Error(t, err)
	})
}

func TestDatabase_Disconnect(t *testing.T) {
	t.Run("Should disconnect successfully with no databases", func(t *testing.T) {
		db := &Database{
			primaryDBs: []*sqlx.DB{},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		err := db.Disconnect()
		
		assert.NoError(t, err)
	})
}

func TestDatabase_WriteDB(t *testing.T) {
	t.Run("Should panic when no primary database configured", func(t *testing.T) {
		db := &Database{
			primaryDBs: []*sqlx.DB{},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		assert.Panics(t, func() {
			db.WriteDB()
		})
	})

	t.Run("Should return single primary database", func(t *testing.T) {
		primaryDB := &sqlx.DB{}
		db := &Database{
			primaryDBs: []*sqlx.DB{primaryDB},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		result := db.WriteDB()
		
		assert.Equal(t, primaryDB, result)
	})

	t.Run("Should return random primary database when multiple available", func(t *testing.T) {
		primaryDB1 := &sqlx.DB{}
		primaryDB2 := &sqlx.DB{}
		db := &Database{
			primaryDBs: []*sqlx.DB{primaryDB1, primaryDB2},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		result := db.WriteDB()
		
		// Verifica se o resultado é um dos bancos primários
		assert.True(t, result == primaryDB1 || result == primaryDB2)
	})
}

func TestDatabase_ReadOnlyDB(t *testing.T) {
	t.Run("Should return WriteDB when no replica databases", func(t *testing.T) {
		primaryDB := &sqlx.DB{}
		db := &Database{
			primaryDBs: []*sqlx.DB{primaryDB},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		result := db.ReadOnlyDB()
		
		assert.Equal(t, primaryDB, result)
	})

	t.Run("Should return single replica database", func(t *testing.T) {
		primaryDB := &sqlx.DB{}
		replicaDB := &sqlx.DB{}
		db := &Database{
			primaryDBs: []*sqlx.DB{primaryDB},
			replicaDBs: []*sqlx.DB{replicaDB},
			apm:        nil,
		}
		
		result := db.ReadOnlyDB()
		
		assert.Equal(t, replicaDB, result)
	})

	t.Run("Should return random replica database when multiple available", func(t *testing.T) {
		primaryDB := &sqlx.DB{}
		replicaDB1 := &sqlx.DB{}
		replicaDB2 := &sqlx.DB{}
		db := &Database{
			primaryDBs: []*sqlx.DB{primaryDB},
			replicaDBs: []*sqlx.DB{replicaDB1, replicaDB2},
			apm:        nil,
		}
		
		result := db.ReadOnlyDB()
		
		// Verifica se o resultado é um dos bancos de réplica
		assert.True(t, result == replicaDB1 || result == replicaDB2)
	})
}

func TestNewRepository(t *testing.T) {
	t.Run("Should create repository with correct table name and primary key", func(t *testing.T) {
		db := &Database{
			primaryDBs: []*sqlx.DB{},
			replicaDBs: []*sqlx.DB{},
			apm:        nil,
		}
		
		repo := NewRepository[TestEntity](db)
		
		assert.NotNil(t, repo)
		assert.Equal(t, db, repo.DB)
		assert.Equal(t, "test_entities", repo.TableName)
		assert.Equal(t, "id", repo.PrimaryKey)
	})
}

func TestDatabaseOptions(t *testing.T) {
	t.Run("Should create DatabaseOptions struct", func(t *testing.T) {
		opts := DatabaseOptions{
			Dsn:  "postgres://user:pass@localhost/db",
			Type: "primary",
		}
		
		assert.Equal(t, "postgres://user:pass@localhost/db", opts.Dsn)
		assert.Equal(t, "primary", opts.Type)
	})
}

func TestDatabaseMigrationsOptions(t *testing.T) {
	t.Run("Should create DatabaseMigrationsOptions struct", func(t *testing.T) {
		config := &postgres.Config{}
		opts := DatabaseMigrationsOptions{
			Dir:    "./migrations",
			Config: config,
		}
		
		assert.Equal(t, "./migrations", opts.Dir)
		assert.Equal(t, config, opts.Config)
	})

	t.Run("Should allow nil Config", func(t *testing.T) {
		opts := DatabaseMigrationsOptions{
			Dir:    "./migrations",
			Config: nil,
		}
		
		assert.Equal(t, "./migrations", opts.Dir)
		assert.Nil(t, opts.Config)
	})
}

func TestEntity_Interface(t *testing.T) {
	t.Run("TestEntity should implement Entity interface", func(t *testing.T) {
		var entity Entity = TestEntity{
			ID:   1,
			Name: "Test",
		}
		
		assert.Equal(t, "test_entities", entity.TableName())
		assert.Equal(t, "id", entity.PrimaryKey())
	})
}

// Teste básico para verificar se as structs podem ser instanciadas
func TestFluxGo_DatabaseIntegration(t *testing.T) {
	t.Run("Should create DatabaseOptions with valid values", func(t *testing.T) {
		opts := DatabaseOptions{
			Dsn:  "postgres://user:pass@localhost/db?sslmode=disable",
			Type: "primary",
		}
		
		assert.Equal(t, "postgres://user:pass@localhost/db?sslmode=disable", opts.Dsn)
		assert.Equal(t, "primary", opts.Type)
	})
}


// Benchmark para testar performance da seleção aleatória de banco
func BenchmarkDatabase_WriteDB(b *testing.B) {
	primaryDBs := make([]*sqlx.DB, 10)
	for i := range primaryDBs {
		primaryDBs[i] = &sqlx.DB{}
	}
	
	db := &Database{
		primaryDBs: primaryDBs,
		replicaDBs: []*sqlx.DB{},
		apm:        nil,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.WriteDB()
	}
}

func BenchmarkDatabase_ReadOnlyDB(b *testing.B) {
	primaryDBs := make([]*sqlx.DB, 5)
	replicaDBs := make([]*sqlx.DB, 10)
	
	for i := range primaryDBs {
		primaryDBs[i] = &sqlx.DB{}
	}
	for i := range replicaDBs {
		replicaDBs[i] = &sqlx.DB{}
	}
	
	db := &Database{
		primaryDBs: primaryDBs,
		replicaDBs: replicaDBs,
		apm:        nil,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.ReadOnlyDB()
	}
}