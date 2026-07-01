# CLAUDE.md - FluxGo Framework

## What is FluxGo

FluxGo is an open-source Go framework for building production-ready applications with a pre-integrated stack. It wraps common enterprise components (HTTP, database, cache, messaging, observability) into a cohesive, opinionated framework powered by dependency injection (Uber fx).

**Import path:** `github.com/MMortari/FluxGo`
**Go version:** 1.25+

## Core Components

| Component | Underlying Tech | Init Method |
|-----------|----------------|-------------|
| HTTP Server | Fiber v2 | `AddHttp(opts, configFn)` |
| Database | sqlx + PostgreSQL (lib/pq) | `AddDatabase(opts)` |
| Cache | go-redis v9 | `AddRedis(opts)` |
| Messaging | Sarama (Apache Kafka) | `AddKafka(opts)` |
| Cron Jobs | gocron v2 | `AddCron()` |
| APM/Tracing | OpenTelemetry | `AddApm()` |
| Logging | slog + OpenTelemetry bridge | `ConfigLogger(opts)` |
| Metrics | Prometheus | `AddPrometheus()` |
| AI Tools | Ollama + JSON Schema | `AddTools()` |
| gRPC Server | google.golang.org/grpc | `AddGrpc(opts)` |
| DI Container | Uber fx | Built-in |

## Project Architecture

FluxGo uses a modular architecture. Each feature is a `FluxModule` with handlers, routes, DTOs, and consumers.

### Recommended directory structure for apps using FluxGo

```
project/
├── main.go                          # Minimal entry point
├── config/
│   └── env.go                       # Typed env config struct
├── modules/
│   └── {module_name}/
│       ├── module.go                # FluxModule definition
│       ├── handlers/{action}.go     # Business logic handlers
│       ├── dto/{action}.go          # Request/Response DTOs
│       └── tests/{action}_test.go   # Tests
├── shared/
│   ├── entities/{entity}.go         # Domain entities (implement Entity interface)
│   ├── repositories/{entity}.go     # Data access layer
│   ├── database/migrations/         # SQL migration files
│   ├── http/main.go                 # HTTP server factory
│   └── module/main.go               # Root module (wires everything)
└── cmd/migrations/run.go            # Migration runner
```

## Key Interfaces

```go
// Entity - domain model for database operations
type Entity interface {
    TableName() string
    PrimaryKey() string
}

// ICache - cache abstraction (Redis implements this)
type ICache interface {
    Get(ctx context.Context, key string) *string
    Store(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Invalidate(ctx context.Context, keys []string) error
}

// ToolsInterface - AI tool integration
type ToolsInterface interface {
    Name() string
    Description() string
    Schema() ToolsSchema
    ExecuteTool(ctx context.Context, raw json.RawMessage) (json.RawMessage, error)
}
```

## Naming Conventions

| What | Pattern | Example |
|------|---------|---------|
| Handler struct | `Handler{Action}{Entity}` | `HandlerGetUser` |
| Handler factory | `{HandlerName}Start` | `HandlerGetUserStart` |
| Repository struct | `{Entity}Repository` | `UserRepository` |
| Repository factory | `{Entity}RepositoryStart` | `UserRepositoryStart` |
| DTO request | `{Action}Req` | `GetUserReq` |
| DTO response | `{Action}Res` | `GetUserRes` |
| Module function | `Module()` | Returns `*fluxgo.FluxModule` |
| Migration files | `{version}_{name}.up.sql` / `.down.sql` | `0_init.up.sql` |

## How to Build an App with FluxGo

### 1. Parse environment config

```go
env := fluxgo.ParseEnv[config.Env](fluxgo.EnvOptions{
    LoadFromFile: fluxgo.Pointer(".env"),
    Validate:     true,
})
```

Env struct must embed `fluxgo.Env` and use `env:` + `validate:` struct tags.

### 2. Initialize FluxGo and add components

```go
flux := fluxgo.New(fluxgo.FluxGoConfig{
    Name: "MyApp", Version: "1.0.0", Env: &env.Env,
})
flux.AddApm()
flux.ConfigLogger(fluxgo.LoggerOptions{Type: "console", Level: "info"})
flux.AddDatabase(fluxgo.DatabaseOptions{
    Instances: []fluxgo.DatabaseConn{
        {Dsn: env.Database.Dsn},                     // primary (default)
        {Dsn: env.Database.ReplicaDsn, Type: "replica"}, // optional read replica
    },
})
flux.AddRedis(fluxgo.RedisOptions{Options: redis.Options{Addr: env.Redis.Addr}})
flux.AddKafka(fluxgo.KafkaOptions{...})
flux.AddCron()
flux.AddHttp(fluxgo.HttpOptions{Port: 3333}, func(data fluxgo.HttpConfigData) {
    data.CreateRouter("/public")
    data.CreateRouter("/api")
})
flux.AddTools()
```

### 3. Create an entity

```go
type User struct {
    ID   string `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
}
func (u User) TableName() string  { return "user" }
func (u User) PrimaryKey() string { return "id" }
```

### 4. Create a repository

```go
type UserRepository struct {
    fluxgo.Repository[entities.User]
}

func UserRepositoryStart(db *fluxgo.Database) *UserRepository {
    return &UserRepository{*fluxgo.NewRepository[entities.User](db)}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*entities.User, error) {
    ctx, span := r.StartSpan(ctx)
    defer span.End()
    var user entities.User
    err := r.DB.ReadOnlyDB().GetContext(ctx, &user, "SELECT * FROM \"user\" WHERE id = $1", id)
    if err == sql.ErrNoRows { return nil, nil }
    if err != nil { span.SetError(err); return nil, err }
    return &user, nil
}
```

- Use `ReadOnlyDB()` / `ReadOnlyDBNamed()` for reads
- Use `WriteDB()` / `WriteDBNamed()` for writes
- Always use `StartSpan` for tracing

### 5. Create a handler

```go
type HandlerGetUser struct {
    repository *repositories.UserRepository
}

func HandlerGetUserStart(repo *repositories.UserRepository) *HandlerGetUser {
    return &HandlerGetUser{repository: repo}
}

func (h *HandlerGetUser) Execute(ctx context.Context, id string) (*dto.GetUserRes, *fluxgo.GlobalError) {
    user, err := h.repository.GetByID(ctx, id)
    if err != nil { return nil, fluxgo.ErrorInternalError("Error getting user") }
    if user == nil { return nil, fluxgo.ErrorNotFound("User not found") }
    return &dto.GetUserRes{User: *user}, nil
}
```

Handlers return `(*DTO, *fluxgo.GlobalError)`. Available error helpers:
- `fluxgo.ErrorInternalError(msg)`
- `fluxgo.ErrorNotFound(msg)`
- `fluxgo.ErrorBadRequest(msg)`
- `fluxgo.ErrorUnauthorized(msg)`

### 6. Create a module

```go
func Module() *fluxgo.FluxModule {
    mod := fluxgo.Module("user")
    mod.AddHandler(handlers.HandlerGetUserStart)

    // HTTP route with cache
    mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
        return mod.HttpRoute(f, "/public", "GET", "/user/:id", fluxgo.RouteIncome{
            Cache: redis, CacheTTL: time.Hour,
        }, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
            resp, err := handler.Execute(c.UserContext(), c.Params("id"))
            if err != nil { return nil, err }
            return &fluxgo.GlobalResponse{Content: resp, Status: 200}, nil
        })
    })

    // POST with body parsing, validation, and cache invalidation
    mod.AddRoute(func(f *fluxgo.FluxGo, redis *fluxgo.Redis, handler *handlers.HandlerGetUser) error {
        return mod.HttpRoute(f, "/api", "POST", "/users", fluxgo.RouteIncome{
            Entity: dto.CreateUserReq{}, FromBody: true, Validate: true,
            Cache: redis, CacheInvalidate: []string{"/public/user"},
        }, func(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
            req := income.(*dto.CreateUserReq)
            // handle creation...
        })
    })

    // Cron job
    mod.AddRoute(func(cron *fluxgo.Cron, logger *fluxgo.Logger) error {
        return mod.CronRoute(cron, "* * * * *", func(ctx context.Context) error {
            logger.Info("cron ran")
            return nil
        })
    })

    // Kafka consumer
    mod.AddRoute(func(f *fluxgo.FluxGo, kafka *fluxgo.Kafka) error {
        return mod.TopicConsume(kafka, "user.created", func(ctx context.Context, data []byte) error {
            // process message
            return nil
        })
    })

    return mod
}
```

### 7. Register module and run

```go
flux.AddDependency(repositories.UserRepositoryStart)
flux.AddModule(user.Module())
flux.Run() // blocking
```

## RouteIncome Options

| Field | Type | Description |
|-------|------|-------------|
| `Entity` | `EntityData` | Struct to parse request data into |
| `FromBody` | `bool` | Parse from request body (JSON) |
| `FromQuery` | `bool` | Parse from query string |
| `FromParam` | `bool` | Parse from URL params |
| `FromHeader` | `bool` | Parse from headers |
| `Validate` | `bool` | Run validation (uses `validate:` struct tags) |
| `Cache` | `ICache` | Cache implementation (usually Redis) |
| `CacheTTL` | `time.Duration` | Cache TTL |
| `CacheInvalidate` | `[]string` | Route prefixes to invalidate on this request |

## Response Types

```go
// Success response
type GlobalResponse struct {
    Status  int         `json:"status"`
    Content interface{} `json:"content"`
}

// Error response
type GlobalError struct {
    Message     string `json:"message,omitempty"`
    Code        string `json:"code"`
    Status      int    `json:"-"`
    Success     bool   `json:"success"`
    Errors      any    `json:"errors,omitempty"`
    UserMessage string `json:"user_message,omitempty"`
}
```

## Testing

```go
func TestGetUser(t *testing.T) {
    fxApp, http := module.Module().GetTestApp(t)
    defer fxApp.RequireStart().RequireStop()

    status, body := fluxgo.RunTestRequest(http, "GET", "/public/user", nil, nil)
    assert.Equal(t, 200, status)
    assert.NotNil(t, body["user"])

    user := fluxgo.ConvertToMap(body["user"])
    assert.Equal(t, "John", user["name"])
}
```

- `RunTestRequest(http, method, path, body, headers)` -> `(int, map[string]interface{})`
- `RunTestRequestRaw(http, method, path, body, headers)` -> `(int, []byte)`
- Run tests with `ENV=test go test ./...`

## Database Migrations

```go
flux.RunMigrations(ctx, fluxgo.DatabaseMigrationsOptions{Dir: "shared/database/migrations"})
flux.RunSeeds(ctx, seedsSQL)
```

Migration files: `{version}_{name}.up.sql` and `{version}_{name}.down.sql`

## Kafka Usage

```go
// Produce
kafka.ProduceMessageJson(ctx, "topic", payload, nil)

// Consume (registered via module)
mod.TopicConsume(kafka, "topic", func(ctx context.Context, data []byte) error { ... })
```

## gRPC Usage

```go
// Setup — alongside AddHttp
flux.AddGrpc(fluxgo.GrpcOptions{
    Port:       50051,
    Reflection: true, // enables grpcurl/Postman
    Interceptors: []grpc.UnaryServerInterceptor{
        myAuthInterceptor(),
    },
})

// Handler — implements proto-generated service interface + GrpcHandlerInterface
type HandlerUserGrpc struct {
    userpb.UnimplementedUserServiceServer
    repository *repositories.UserRepository
}

func HandlerUserGrpcStart(repo *repositories.UserRepository) *HandlerUserGrpc {
    return &HandlerUserGrpc{repository: repo}
}

func (h *HandlerUserGrpc) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
    // business logic
}

// RegisterGrpc is the FluxGo hook — called once on startup
func (h *HandlerUserGrpc) RegisterGrpc(server *grpc.Server) {
    userpb.RegisterUserServiceServer(server, h)
}

// Module registration
fluxgo.Module("user").
    AddHandler(handlers.HandlerUserGrpcStart).
    Route(
        fluxgo.GrpcDef[handlers.HandlerUserGrpc](),
    )
```

Proto files live in `proto/` and generated code in `shared/pb/`. Regenerate with:

```bash
protoc \
  --go_out=shared/pb --go_opt=paths=source_relative \
  --go-grpc_out=shared/pb --go-grpc_opt=paths=source_relative \
  -I proto proto/user/user.proto
# or: make proto
```

## AI Tools Integration

```go
type MyTool struct{}
func (t *MyTool) Name() string        { return "my_tool" }
func (t *MyTool) Description() string  { return "Does something" }
func (t *MyTool) Schema() fluxgo.ToolsSchema { return fluxgo.ToolParseSchema(MyParams{}) }
func (t *MyTool) ExecuteTool(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) { ... }

// Register
flux.AddTools()
flux.AddDependency(func(tools *fluxgo.Tools) { tools.AddTool(&MyTool{}) })

// Get Ollama-compatible tool definitions
tools.GetOllamaTools()
```

## Build & Run

```bash
go run ./main.go          # Run app
ENV=test go test ./...    # Run tests
go run cmd/migrations/run.go  # Run migrations
```

## Common Infrastructure (Docker Compose)

- **PostgreSQL** on port 5432 (required for `AddDatabase`)
- **Redis** on port 6379 (required for `AddRedis`)
- **Jaeger** on ports 4317/4318/16686 (required for `AddApm` with OTLP exporter)
- **Kafka** on port 9092 (required for `AddKafka`)

## Important Implementation Notes

- All dependency injection is handled by Uber fx. Constructor functions (factories ending in `Start`) are registered via `AddDependency()` or `AddHandler()`.
- The `FluxGo.Run()` call is blocking and manages the full application lifecycle (start/stop of all components).
- Repositories should use `StartSpan()` for APM tracing on every database operation.
- Use `ReadOnlyDB()` for reads and `WriteDB()` for writes to properly route to primary/replica databases.
- Cache keys are auto-generated from the request path. `CacheInvalidate` takes route prefix patterns.
- The `Env` struct uses `caarlos0/env` tags for parsing and `go-playground/validator` tags for validation.
- Logger type options: `"console"`, `"file"`, `"otel"`. In tests, use `"console"`.
