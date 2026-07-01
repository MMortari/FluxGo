# Padrões de Arquitetura FluxGo

Este documento descreve os padrões de arquitetura e estrutura de código recomendados com esse pacote.

## 📁 Estrutura de Diretórios

```
project/
├── main.go                    # Ponto de entrada da aplicação
├── Makefile                   # Comandos úteis (run, test, migrations)
├── api.http                   # Arquivo de testes HTTP (REST Client)
├── go.mod                     # Dependências Go
├── cmd/
│   └── migrations/
│       └── run.go            # Script de execução de migrações
├── config/
│   └── env.go                # Configuração de variáveis de ambiente
├── logs/                      # Diretório de logs
│   └── out.log
├── modules/                   # Módulos de negócio (features)
│   └── {module_name}/
│       ├── module.go         # Configuração do módulo FluxGo
│       ├── handlers/         # Handlers de requisições HTTP
│       │   └── {action}.go
│       ├── dto/              # Data Transfer Objects
│       │   └── {action}.go
│       └── tests/            # Testes do módulo
│           └── {action}_test.go
├── proto/                     # Fontes .proto (versionadas no git)
│   └── {service}/
│       └── {service}.proto
└── shared/                    # Recursos compartilhados
    ├── database/
    │   └── migrations/       # Migrações SQL
    │       ├── {version}_{name}.up.sql
    │       └── {version}_{name}.down.sql
    ├── entities/             # Entidades de domínio
    │   └── {entity}.go
    ├── pb/                   # Código gerado pelo protoc (não editar)
    │   └── {service}/
    │       ├── {service}.pb.go
    │       └── {service}_grpc.pb.go
    ├── repositories/          # Camada de acesso a dados
    │   └── {entity}.go
    ├── http/                 # Configuração HTTP
    │   └── main.go
    └── module/               # Módulo principal FluxGo
        └── main.go
```

## 🏗️ Arquitetura

### Camadas da Aplicação

1. **Handlers** (`modules/{module}/handlers/`)
   - Recebem requisições HTTP
   - Orquestram a lógica de negócio
   - Retornam DTOs ou erros globais

2. **DTOs** (`modules/{module}/dto/`)
   - Objetos de transferência de dados
   - Estruturas de resposta/requisição
   - Serialização JSON

3. **Repositories** (`shared/repositories/`)
   - Acesso a dados
   - Queries SQL
   - Abstração do banco de dados

4. **Entities** (`shared/entities/`)
   - Modelos de domínio
   - Estruturas de dados principais
   - Métodos auxiliares (TableName, PrimaryKey)

## 📝 Padrões de Código

### 1. Main Entry Point (`main.go`)

```go
package main

import (
	"github.com/MMortari/FluxGo/example/full/shared/module"
)

func main() {
	flux := module.Module()
	flux.Run()
}
```

**Padrão**: Entry point mínimo que inicializa o módulo principal e executa.

---

### 2. Módulo Principal (`shared/module/main.go`)

```go
package module

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/config"
	"github.com/MMortari/FluxGo/example/full/modules/user"
	"github.com/MMortari/FluxGo/example/full/shared/http"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"github.com/redis/go-redis/v9"
)

func Module() *fluxgo.FluxGo {
	// 1. Parse de variáveis de ambiente
	env := fluxgo.ParseEnv[config.Env](fluxgo.EnvOptions{
		LoadFromFile: fluxgo.Pointer(".env.development"),
		Validate: true,
	})

	// 2. Criação do FluxGo com configurações básicas
	flux := fluxgo.New(fluxgo.FluxGoConfig{
		Name: "Teste Full",
		Version: "1",
		Env: &env.Env,
		Debugger: true,
	})

	// 3. Configuração de APM (Application Performance Monitoring)
	flux.AddApm(fluxgo.ApmOptions{
		CollectorURL: env.Apm.CollectorUrl,
		Exporter: env.Apm.Exporter,
	})

	// 4. Configuração de Logger
	flux.ConfigLogger(fluxgo.LoggerOptions{
		Type: env.Logger.Type,
		Level: env.Logger.Level,
		LogFilePath: env.Logger.FilePath,
	})

	// 5. Adição de dependências
	flux.AddDependency(func() *config.Env { return &env })

	// 6. Configuração de banco de dados (primário simples)
	flux.AddDatabase(fluxgo.DatabaseOptions{
		Instances: []fluxgo.DatabaseConn{
			{Dsn: env.Database.Dsn}, // Type vazio ou diferente de "replica" é tratado como primário
		},
	})

	// 7. Configuração de Redis
	flux.AddRedis(fluxgo.RedisOptions{
		Options: redis.Options{Addr: env.Redis.Addr},
	})

	// 8. Configuração de Cron Jobs
	flux.AddCron()

	// 9. Configuração de HTTP Server
	flux.AddHttp(http.GetHttp(flux.GetApm()))

	// 10. Registro de repositories como dependências
	flux.AddDependency(repositories.UserRepositoryStart)

	// 11. Registro de módulos de negócio
	flux.AddModule(user.Module())

	return flux
}
```

**Padrão**: Configuração centralizada de todos os serviços e dependências.

- **Banco de dados**:
  - Uso de `DatabaseOptions{Instances: []DatabaseConn{...}}`
  - Cada item de `Instances` representa uma conexão de banco
  - `Type: "replica"` indica banco de leitura; ausência desse valor indica banco primário
  - Métodos de acesso: `WriteDB()/WriteDBNamed()` para escrita e `ReadOnlyDB()/ReadOnlyDBNamed()` para leitura

---

### 3. Configuração de Ambiente (`config/env.go`)

```go
package config

import fluxgo "github.com/MMortari/FluxGo"

type Env struct {
	fluxgo.Env
	Database struct {
		Dsn string `env:"DATABASE_DSN" validate:"required"`
	}
	Redis struct {
		Addr string `env:"REDIS_ADDR" validate:"required"`
	}
	Logger struct {
		Type     string `env:"LOGGER_TYPE" validate:"required"`
		Level    string `env:"LOGGER_LEVEL" validate:"required"`
		FilePath string `env:"LOGGER_FILE_PATH" validate:"required"`
	}
	Apm struct {
		Exporter     string `env:"APM_EXPORTER" validate:"required"`
		CollectorUrl string `env:"APM_COLLECTOR_URL" validate:"required"`
	}
}
```

**Padrão**:

- Estrutura tipada para variáveis de ambiente
- Validação automática com tags `validate:"required"`
- Agrupamento lógico por serviço

---

### 4. Configuração HTTP (`shared/http/main.go`)

```go
package http

import (
	fluxgo "github.com/MMortari/FluxGo"
	"github.com/gofiber/fiber/v2"
)

func GetHttp(apm *fluxgo.Apm) *fluxgo.Http {
	http := fluxgo.NewHttp(fluxgo.HttpOptions{
		Port: 3333,
		LogRequest: true,
		Apm: apm,
	})
	http.CreateRouter("/public", middlewareExample(apm))

	return http
}

func middlewareExample(apm *fluxgo.Apm) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}
```

**Padrão**:

- Função factory para criação do servidor HTTP
- Criação de rotas com prefixo (ex: `/public`)
- Middlewares configuráveis

---

### 5. Módulo de Negócio (`modules/{module}/module.go`)

```go
package user

import (
	"context"
	"log"
	"time"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/handlers"
	"github.com/gofiber/fiber/v2"
)

func Module() *fluxgo.FluxModule {
	mod := fluxgo.Module("user")

	// Registro de handlers (injeção de dependência)
	mod.AddHandler(handlers.HandlerGetUserStart)

	// Rota GET simples
	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "GET", "/user", fluxgo.RouteIncome{
			Cache:    redis,
			CacheTTL: time.Hour,
		}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			resp, err := handler.Execute(c.UserContext(), "")
			if err != nil {
				return nil, err
			}
			return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
		})
	})

	// Rota GET com parâmetro
	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "GET", "/user/:id_user", fluxgo.RouteIncome{
			Cache:    redis,
			CacheTTL: time.Hour,
		}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			resp, err := handler.Execute(c.UserContext(), "")
			if err != nil {
				return nil, err
			}
			return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
		})
	})

	// Rota POST com invalidação de cache
	mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
		return mod.HttpRoute(f, "/public", "POST", "/refresh", fluxgo.RouteIncome{
			Cache:           redis,
			CacheInvalidate: []string{"/public/user"},
		}, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
			resp, err := handler.Execute(c.UserContext(), "")
			if err != nil {
				return nil, err
			}
			return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
		})
	})

	// Rota Cron Job
	mod.AddRoute(func(cron *fluxgo.Cron, logger *fluxgo.Logger, handler *handlers.HandlerGetUser) error {
		return mod.CronRoute(cron, "* * * * *", func(ctx context.Context) error {
			logger.Infoln("Cron executed")
			log.Println("Cron executed")
			return nil
		})
	})

	return mod
}
```

**Padrão**:

- Função `Module()` retorna `*fluxgo.FluxModule`
- Registro de handlers com `AddHandler()`
- Rotas HTTP com `AddRoute()` e `HttpRoute()`
- Suporte a cache Redis com TTL
- Invalidação de cache
- Cron jobs com `CronRoute()`
- Injeção de dependências automática

---

### 6. Handler (`modules/{module}/handlers/{action}.go`)

```go
package handlers

import (
	c "context"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/modules/user/dto"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
)

type HandlerGetUser struct {
	repository *repositories.UserRepository
}

// Função de inicialização (factory) - padrão: {HandlerName}Start
func HandlerGetUserStart(repository *repositories.UserRepository) *HandlerGetUser {
	return &HandlerGetUser{repository: repository}
}

// Método Execute - padrão de execução
func (h *HandlerGetUser) Execute(ctx c.Context, idUser string) (*dto.GetUserRes, *fluxgo.GlobalError) {
	user, err := h.repository.GetUser(ctx)
	if err != nil {
		return nil, fluxgo.ErrorInternalError("Error to get user")
	}
	if user == nil {
		return nil, fluxgo.ErrorNotFound("User not found")
	}
	return &dto.GetUserRes{User: *user}, nil
}
```

**Padrão**:

- Struct do handler com dependências
- Função factory `{HandlerName}Start` para inicialização
- Método `Execute()` que recebe context e parâmetros
- Retorno: `(*DTO, *GlobalError)`
- Uso de erros globais do FluxGo (`ErrorInternalError`, `ErrorNotFound`)

---

### 7. DTO (`modules/{module}/dto/{action}.go`)

```go
package dto

import "github.com/MMortari/FluxGo/example/full/shared/entities"

type GetUserRes struct {
	User entities.User `json:"user"`
}
```

**Padrão**:

- Estruturas simples para transferência de dados
- Tags JSON para serialização
- Composição com entities quando apropriado
- Nomenclatura: `{Action}{Req|Res}`

---

### 8. Repository (`shared/repositories/{entity}.go`)

```go
package repositories

import (
	"context"
	"database/sql"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/shared/entities"
)

type UserRepository struct {
	fluxgo.Repository[entities.User]
}

// Função factory - padrão: {Entity}RepositoryStart
func UserRepositoryStart(db *fluxgo.Database) *UserRepository {
	return &UserRepository{*fluxgo.NewRepository[entities.User](db)}
}

func (r *UserRepository) GetUser(ctx context.Context) (*entities.User, error) {
	ctx, span := r.StartSpan(ctx)
	defer span.End()

	var user entities.User

	err := r.DB.ReadOnlyDB().GetContext(ctx, &user, "SELECT '299f3dcd-42f3-46c1-89d5-603c78a78f50' as id, 'John' AS name")
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		span.SetError(err)
		return nil, err
	}

	return &user, nil
}
```

**Padrão**:

- Struct que embede `fluxgo.Repository[Entity]`
- Função factory `{Entity}RepositoryStart` que recebe `*fluxgo.Database`
- Métodos recebem `context.Context`
- Uso de `StartSpan()` para tracing (APM)
- Tratamento de `sql.ErrNoRows` retornando `nil, nil`
- Uso de `ReadOnlyDB()` para queries de leitura
- `SetError()` no span em caso de erro

---

### 9. Entity (`shared/entities/{entity}.go`)

```go
package entities

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (u User) TableName() string {
	return "user"
}

func (u User) PrimaryKey() string {
	return "id"
}
```

**Padrão**:

- Struct simples representando entidade de domínio
- Tags JSON para serialização
- Métodos `TableName()` e `PrimaryKey()` para integração com Repository

---

### 10. Migrações (`cmd/migrations/run.go`)

```go
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

	env := fluxgo.ParseEnv[config.Env](fluxgo.EnvOptions{
		LoadFromFile: fluxgo.Pointer(".env.development"),
		Validate: true,
	})

	flux := fluxgo.
		New(fluxgo.FluxGoConfig{Name: "Migrations"}).
		AddApm(fluxgo.ApmOptions{
			CollectorURL: env.Apm.CollectorUrl,
			Exporter:     env.Apm.Exporter,
		}).
		AddDatabase(fluxgo.DatabaseOptions{
			Instances: []fluxgo.DatabaseConn{
				{Dsn: env.Database.Dsn},
			},
		})

	if err := flux.RunMigrations(ctx, fluxgo.DatabaseMigrationsOptions{
		Dir: "shared/database/migrations",
	}); err != nil {
		panic(err)
	}

	seeds := `
		INSERT INTO "user" (name) VALUES ('John Doe');
	`

	if err := flux.RunSeeds(ctx, seeds); err != nil {
		panic(err)
	}
}
```

**Padrão**:

- Script separado para execução de migrações
- Context com timeout
- Execução de migrações com `RunMigrations()`
- Execução de seeds com `RunSeeds()`
- Reutiliza a mesma configuração de banco de dados do módulo principal (`DatabaseOptions{Instances: []DatabaseConn{...}}`)

---

### 11. Migrações SQL (`shared/database/migrations/`)

**Arquivo UP** (`{version}_{name}.up.sql`):

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE "user" (
  id_user uuid default gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  PRIMARY KEY(id_user)
);
```

**Arquivo DOWN** (`{version}_{name}.down.sql`):

```sql
DROP TABLE "user";
```

**Padrão**:

- Nomenclatura: `{version}_{name}.up.sql` e `{version}_{name}.down.sql`
- Versão numérica no início (ex: `0_init`)
- Arquivo UP para criação/alteração
- Arquivo DOWN para reversão

---

### 12. Testes (`modules/{module}/tests/{action}_test.go`)

```go
package handlers

import (
	"testing"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/shared/module"
	"github.com/stretchr/testify/assert"
)

func TestOnboardContact(t *testing.T) {
	fx, app := module.Module().GetTestApp(t)
	defer fx.RequireStart().RequireStop()

	endpoint := "/public/user"
	successHttpCode := 200

	t.Run("Success", func(t *testing.T) {
		status, body := fluxgo.RunTestRequest(app, "GET", endpoint, nil, nil)

		assert.Equalf(t, successHttpCode, status, "Invalid status code")
		assert.NotNilf(t, body["user"], "Invalid body response")

		user := fluxgo.ConvertToMap(body["user"])
		assert.Equalf(t, "299f3dcd-42f3-46c1-89d5-603c78a78f50", user["id"], "Invalid body response")
		assert.Equalf(t, "John", user["name"], "Invalid body response")
	})
}
```

**Padrão**:

- Uso de `GetTestApp()` para obter app de teste
- `RequireStart()` e `RequireStop()` para lifecycle
- `RunTestRequest()` para requisições HTTP de teste
- `ConvertToMap()` para conversão de respostas
- Uso de `testify/assert` para asserções

---

### 13. Docker Compose (`docker-compose.yml`)

```yaml
version: "3.8"
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "4318:4318"
      - "4317:4317"
    environment:
      - LOG_LEVEL=debug

  database:
    image: postgres:16
    container_name: database
    ports:
      - 5435:5432
    environment:
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=flux

  redis:
    container_name: redis
    image: redis:7.4-alpine
    restart: always
    ports:
      - 6398:6379
    environment:
      - ALLOW_EMPTY_PASSWORD=yes
```

**Padrão**:

- **Jaeger (APM/Tracing)**:
  - Inclua o serviço `jaeger` quando o projeto usar APM via OTEL (`AddApm`).
  - Exponha as portas `16686` (UI) e `4317`/`4318` (OTLP).
  - Ajuste `CollectorURL` nas envs para apontar para o Jaeger do compose.
- **Database (Postgres)**:
  - Use o serviço `database` quando o projeto depender de banco relacional.
  - Mapeie a porta externa conforme necessidade (ex.: `5435:5432`) e mantenha as envs compatíveis com `DATABASE_DSN`.
  - No `DATABASE_DSN`, aponte o host para `database` (nome do serviço).
  - Para múltiplas instâncias/replicas, crie novos serviços (ex.: `database_replica`) e ajuste `DatabaseOptions.Instances`.
- **Redis**:
  - Inclua o serviço `redis` apenas se a aplicação usar cache/filas com Redis (`AddRedis`).
  - Mapeie a porta externa conforme necessidade (ex.: `6398:6379`) e mantenha as envs compatíveis com `REDIS_ADDR`.
  - No `REDIS_ADDR`, aponte o host para `redis`.
- **Customização por serviço**:
  - Remova serviços não utilizados (ex.: sem Redis, remova o bloco `redis`).
  - Adicione novos serviços de infraestrutura conforme a necessidade do domínio (ex.: filas, storage) e exponha via variáveis de ambiente tipadas na `config.Env`.
  - Mantenha a assinatura de envs consistente com o que o módulo principal espera (por exemplo, `DATABASE_DSN`, `REDIS_ADDR`, `APM_COLLECTOR_URL`).

---

### 14. Makefile

```makefile
run:
	go run ./main.go

test:
	ENV=test go test ./...

migrations:
	go run cmd/migrations/run.go

proto:
	protoc \
		--go_out=shared/pb \
		--go_opt=paths=source_relative \
		--go-grpc_out=shared/pb \
		--go-grpc_opt=paths=source_relative \
		-I proto \
		proto/{service}/{service}.proto
```

**Padrão**:

- Comandos úteis para desenvolvimento
- `run`: executa a aplicação
- `test`: executa testes com variável de ambiente
- `migrations`: executa migrações
- `proto`: regenera código Go a partir dos arquivos `.proto` (requer `protoc`, `protoc-gen-go` e `protoc-gen-go-grpc`)

---

### 15. API HTTP (`api.http`)

```http
GET http://localhost:3333/public/user
content-type: application/json

###
GET http://localhost:3333/public/user/1
content-type: application/json

###
POST http://localhost:3333/public/refresh
content-type: application/json
```

**Padrão**:

- Arquivo para testes manuais de API
- Formato compatível com REST Client (VS Code)
- Separação de requisições com `###`

---

### 16. Handler gRPC (`modules/{module}/handlers/grpc.go`)

```go
package handlers

import (
	"context"

	fluxgo "github.com/MMortari/FluxGo"
	userpb "github.com/MMortari/FluxGo/example/full/shared/pb/user"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HandlerUserGrpc embede UnimplementedUserServiceServer (gerado pelo protoc)
// e implementa fluxgo.GrpcHandlerInterface via RegisterGrpc.
type HandlerUserGrpc struct {
	userpb.UnimplementedUserServiceServer
	repository *repositories.UserRepository
	logger     *fluxgo.Logger
}

func HandlerUserGrpcStart(repository *repositories.UserRepository, logger *fluxgo.Logger) *HandlerUserGrpc {
	return &HandlerUserGrpc{repository: repository, logger: logger}
}

// Método gRPC — assinatura gerada pelo protoc
func (h *HandlerUserGrpc) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	user, err := h.repository.GetUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting user")
	}
	if user == nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}
	return &userpb.GetUserResponse{
		User: &userpb.User{Id: user.ID, Name: user.Name},
	}, nil
}

// RegisterGrpc implementa fluxgo.GrpcHandlerInterface — chamado uma vez no startup
func (h *HandlerUserGrpc) RegisterGrpc(server *grpc.Server) {
	userpb.RegisterUserServiceServer(server, h)
}
```

**Padrão**:

- Struct embede `Unimplemented{Service}Server` do código gerado (garante forward-compatibility)
- Métodos gRPC seguem assinatura gerada pelo protoc: `(ctx, *Req) (*Res, error)`
- `RegisterGrpc` é o único método específico do FluxGo — delega para `Register{Service}Server`
- Erros retornam `status.Errorf(codes.X, msg)` (padrão gRPC), não `*GlobalError`
- Handler separado por transporte: `get-user.go` (HTTP/Cron/Kafka) + `grpc.go` (gRPC)
- Registro no módulo: `AddHandler(HandlerUserGrpcStart)` + `Route(fluxgo.GrpcDef[HandlerUserGrpc]())`

---

### 17. Tools

O projeto inclui um componente `Tools` para registro e exposição de ferramentas programáticas.

- **Visão geral**: Um container que mantém ferramentas que implementam `ToolsInterface` e provê utilitários como `GetOllamaTools()` para gerar definições JSON compatíveis com provedores (ex: `ollama`).
- **Interface mínima**:
  - `Name() string`
  - `Description() string`
  - `Schema() ToolsSchema` (schema JSON das entradas)
  - `ExecuteTool(ctx context.Context, raw json.RawMessage) (any, error)`
- **Como usar**:
  1.  Habilite o container no `Module()` chamando `flux.AddTools()`.
  2.  Registre suas tools via dependência para receber o `*fluxgo.Tools` e chamar `AddTool()`.

```go
flux.AddTools()

flux.AddDependency(func(tools *fluxgo.Tools) error {
		tools.AddTool(&MyTool{})
		return nil
})
```

- **Schema / integração**: use `ToolParseSchema()` para gerar `ToolsSchema` a partir de structs de parâmetros. `GetOllamaTools()` retorna o JSON com as definições prontas para envio ao provedor `ollama`.

Exemplo resumido:

```go
type MyToolParams struct { Query string `json:"query"` }

type MyTool struct{}
func (t *MyTool) Name() string { return "my_tool" }
func (t *MyTool) Description() string { return "Busca algo" }
func (t *MyTool) Schema() ToolsSchema { return ToolParseSchema(MyToolParams{}) }
func (t *MyTool) ExecuteTool(ctx context.Context, raw json.RawMessage) (any, error) { return nil, nil }
```

**Padrões**:

- Registrar `Tools` no início da inicialização do módulo.
- Fornecer schemas para validação/integração com provedores de funções.
- Usar `GetOllamaTools()` para integração com provedores que esperam definições de funções.

### 18. Configuração e Uso do Kafka

### 1. Variáveis de Ambiente

Adicione as variáveis necessárias no seu arquivo de ambiente:

```env
KAFKA_BROKERS=localhost:9092
KAFKA_GROUP_ID=fluxgo-group
KAFKA_TLS_ENABLED=false
```

### 2. Estrutura de Configuração

No seu struct de env (exemplo em `config/env.go`):

```go
Kafka struct {
	Brokers    []string `env:"KAFKA_BROKERS" envSeparator:"," validate:"required"`
	GroupId    string   `env:"KAFKA_GROUP_ID" validate:"required"`
	TlsEnabled bool     `env:"KAFKA_TLS_ENABLED"`
}
```

### 3. Registro do Kafka no Módulo Principal

No `shared/module/main.go`:

```go
flux.AddKafka(fluxgo.KafkaOptions{
	Brokers: env.Kafka.Brokers,
	Auth: fluxgo.KafkaAuth{
		TlsEnabled: env.Kafka.TlsEnabled,
	},
	Consumer: &fluxgo.KafkaConsumerOptions{
		GroupId:    env.Kafka.GroupId,
		AutoCommit: true,
	},
	Producer: &fluxgo.KafkaProducerOptions{
		Acks: sarama.WaitForAll,
	},
})
```

### 4. Produção de Mensagens

Para publicar mensagens em um tópico:

```go
err := kafka.ProduceMessageJson(ctx, "TOPICO", map[string]interface{}{
	"foo": "bar",
}, nil)
```

### 5. Consumo de Mensagens

No seu módulo, registre o consumer:

```go
mod.AddRoute(func(f *fluxgo.FluxGo, kafka *fluxgo.Kafka) error {
	return mod.TopicConsume(kafka, "TOPICO", func(ctx context.Context, data []byte) error {
		// Processa a mensagem
		return nil
	})
})
```

### 6. Observações

- O consumer é gerenciado automaticamente pelo ciclo de vida do FluxGo (start/stop).
- O handler recebe o contexto e o payload da mensagem.
- Para múltiplos tópicos, chame `TopicConsume` para cada um.
- O producer pode ser usado em qualquer handler ou cron.

---

## 🔄 Fluxo de Dados

```
HTTP Request
    ↓
Router (Fiber)
    ↓
Handler (modules/{module}/handlers/)
    ↓
Repository (shared/repositories/)
    ↓
Database
    ↓
Entity (shared/entities/)
    ↓
DTO (modules/{module}/dto/)
    ↓
HTTP Response
```

## 🎯 Convenções de Nomenclatura

- **Módulos**: `modules/{nome_do_modulo}/`
- **Handlers**: `{Action}{Entity}` (ex: `HandlerGetUser`)
- **DTOs**: `{Action}{Req|Res}` (ex: `GetUserRes`)
- **Repositories**: `{Entity}Repository` (ex: `UserRepository`)
- **Entities**: Singular, PascalCase (ex: `User`)
- **Funções Factory**: `{Type}Start` (ex: `HandlerGetUserStart`, `UserRepositoryStart`)
- **Migrações**: `{version}_{nome}.{up|down}.sql`

## 📦 Dependências Principais

- `github.com/MMortari/FluxGo` - Framework principal
- `github.com/gofiber/fiber/v2` - Framework HTTP
- `github.com/redis/go-redis/v9` - Cliente Redis
- `github.com/stretchr/testify` - Biblioteca de testes

## 🚀 Checklist para Novo Módulo

- [ ] Criar diretório `modules/{nome_modulo}/`
- [ ] Criar `module.go` com função `Module()`
- [ ] Criar handlers em `handlers/`
- [ ] Criar DTOs em `dto/`
- [ ] Criar testes em `tests/`
- [ ] Criar/atualizar entity em `shared/entities/`
- [ ] Criar/atualizar repository em `shared/repositories/`
- [ ] Registrar handler no `module.go` com `AddHandler()`
- [ ] Registrar rotas no `module.go` com `AddRoute()` ou `Route(...)`
- [ ] Registrar repository no `shared/module/main.go` com `AddDependency()`
- [ ] Registrar módulo no `shared/module/main.go` com `AddModule()`

### Checklist adicional para gRPC

- [ ] Criar `proto/{service}/{service}.proto` com definição do serviço
- [ ] Gerar código com `make proto` → `shared/pb/{service}/`
- [ ] Criar `modules/{module}/handlers/grpc.go` com handler que embede `Unimplemented{Service}Server`
- [ ] Implementar `RegisterGrpc(server *grpc.Server)` chamando `Register{Service}Server`
- [ ] Registrar handler com `AddHandler({Handler}GrpcStart)`
- [ ] Registrar rota com `Route(fluxgo.GrpcDef[{Handler}Grpc]())`
- [ ] Chamar `flux.AddGrpc(fluxgo.GrpcOptions{Port: 50051})` no `shared/module/main.go`

## 📚 Referências

- Repositório: https://github.com/MMortari/FluxGo/tree/main/example/full
