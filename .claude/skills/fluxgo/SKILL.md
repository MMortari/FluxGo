---
name: fluxgo
description: >
  Implementation guide for FluxGo framework apps. Use when the user asks to create,
  add, or implement any feature in a FluxGo project: new modules, handlers, repositories,
  routes (HTTP, gRPC, Kafka, Cron), entities, DTOs, or migrations.
  Trigger: "implement X", "add module", "create handler", "add route", "new entity", "add gRPC".
---

You are implementing a feature in a **FluxGo** application.

FluxGo is a Go framework for production-ready apps built on Uber fx (DI), Fiber (HTTP), sqlx (DB), go-redis, Sarama (Kafka), gocron, OpenTelemetry, and google.golang.org/grpc.

Import path: `github.com/MMortari/FluxGo`

---

## Step 1 — Read existing project context

Before writing any code, always read:
1. `CLAUDE.md` or `PATTERN.md` if they exist in the project root
2. `config/env.go` — understand the environment config struct
3. `shared/module/main.go` — see what services are already registered
4. `shared/entities/` — see existing domain entities
5. The relevant existing module in `modules/` if modifying one

---

## Step 2 — Identify what to create

Map the user's request to these building blocks:

| What | When |
|------|------|
| Entity | New domain model needed |
| Migration | New/changed table |
| Repository | DB access for entity |
| Handler | Business logic for an action |
| DTO | Request/response struct for handler |
| Module | New feature grouping routes + handlers |
| Route | HTTP / gRPC / Kafka / Cron endpoint |

---

## Step 3 — Implementation patterns

### Entity (`shared/entities/{entity}.go`)

```go
type User struct {
    ID   string `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
}
func (u User) TableName() string  { return "user" }
func (u User) PrimaryKey() string { return "id" }
```

### Repository (`shared/repositories/{entity}.go`)

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
    err := r.DB.ReadOnlyDB().GetContext(ctx, &user, `SELECT * FROM "user" WHERE id = $1`, id)
    if err == sql.ErrNoRows { return nil, nil }
    if err != nil { span.SetError(err); return nil, err }
    return &user, nil
}
```

Rules:
- Always call `r.StartSpan(ctx)` + `defer span.End()`
- `ReadOnlyDB()` for SELECTs, `WriteDB()` for INSERT/UPDATE/DELETE
- Return `nil, nil` on `sql.ErrNoRows`
- Call `span.SetError(err)` before returning errors

### DTO (`modules/{module}/dto/{action}.go`)

```go
// Request
type GetUserReq struct {
    ID string `json:"id" validate:"required,uuid"`
}

// Response
type GetUserRes struct {
    User entities.User `json:"user"`
}
```

### Handler (`modules/{module}/handlers/{action}.go`)

```go
type HandlerGetUser struct {
    repository *repositories.UserRepository
    logger     *fluxgo.Logger
}

func HandlerGetUserStart(repo *repositories.UserRepository, logger *fluxgo.Logger) *HandlerGetUser {
    return &HandlerGetUser{repository: repo, logger: logger}
}

// Core logic — reused by all transports
func (h *HandlerGetUser) Execute(ctx context.Context, req *dto.GetUserReq) (*dto.GetUserRes, *fluxgo.GlobalError) {
    user, err := h.repository.GetByID(ctx, req.ID)
    if err != nil  { return nil, fluxgo.ErrorInternalError("error getting user") }
    if user == nil { return nil, fluxgo.ErrorNotFound("user not found") }
    return &dto.GetUserRes{User: *user}, nil
}

// HTTP transport
func (h *HandlerGetUser) HandleHttp(c *fiber.Ctx, income interface{}) (*fluxgo.GlobalResponse, *fluxgo.GlobalError) {
    res, err := h.Execute(c.UserContext(), income.(*dto.GetUserReq))
    if err != nil { return nil, err }
    return &fluxgo.GlobalResponse{Content: res, Status: 200}, nil
}

// Kafka transport (optional)
func (h *HandlerGetUser) HandleMessage(ctx context.Context, data []byte) error {
    var req dto.GetUserReq
    if err := json.Unmarshal(data, &req); err != nil { return err }
    _, gErr := h.Execute(ctx, &req)
    if gErr != nil { return errors.New(gErr.Message) }
    return nil
}

// Cron transport (optional)
func (h *HandlerGetUser) HandleCron(ctx context.Context) error {
    h.logger.Info("cron executed")
    return nil
}
```

Error helpers: `fluxgo.ErrorInternalError(msg)` | `fluxgo.ErrorNotFound(msg)` | `fluxgo.ErrorBadRequest(msg)` | `fluxgo.ErrorUnauthorized(msg)`

### gRPC Handler (`modules/{module}/handlers/grpc.go`)

Requires proto-generated code in `shared/pb/{service}/`. Generate with:
```bash
protoc --go_out=shared/pb --go_opt=paths=source_relative \
       --go-grpc_out=shared/pb --go-grpc_opt=paths=source_relative \
       -I proto proto/{service}/{service}.proto
```

```go
type HandlerUserGrpc struct {
    userpb.UnimplementedUserServiceServer   // always embed
    repository *repositories.UserRepository
    logger     *fluxgo.Logger
}

func HandlerUserGrpcStart(repo *repositories.UserRepository, logger *fluxgo.Logger) *HandlerUserGrpc {
    return &HandlerUserGrpc{repository: repo, logger: logger}
}

func (h *HandlerUserGrpc) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
    user, err := h.repository.GetByID(ctx, req.GetId())
    if err != nil  { return nil, status.Errorf(codes.Internal, "error getting user") }
    if user == nil { return nil, status.Errorf(codes.NotFound, "user not found") }
    return &userpb.GetUserResponse{User: &userpb.User{Id: user.ID, Name: user.Name}}, nil
}

// RegisterGrpc is the FluxGo hook
func (h *HandlerUserGrpc) RegisterGrpc(server *grpc.Server) {
    userpb.RegisterUserServiceServer(server, h)
}
```

gRPC errors use `status.Errorf(codes.X, msg)` — NOT `*fluxgo.GlobalError`.

### Module (`modules/{module}/module.go`)

```go
func Module() *fluxgo.FluxModule {
    return fluxgo.Module("user").
        AddHandler(
            handlers.HandlerGetUserStart,
            handlers.HandlerUserGrpcStart, // if gRPC
        ).
        Route(
            // HTTP
            fluxgo.GET[handlers.HandlerGetUser]("/public", "/user/:id", fluxgo.RouteIncome{
                Entity: dto.GetUserReq{}, FromParam: true, Validate: true,
                CacheTTL: time.Hour,
            }),
            fluxgo.POST[handlers.HandlerGetUser]("/api", "/users", fluxgo.RouteIncome{
                Entity: dto.CreateUserReq{}, FromBody: true, Validate: true,
                CacheInvalidate: []string{"/public/user"},
            }),

            // gRPC (requires AddGrpc in shared/module/main.go)
            fluxgo.GrpcDef[handlers.HandlerUserGrpc](),

            // Kafka consumer
            fluxgo.TopicDef[handlers.HandlerGetUser]("user.created"),

            // Cron
            fluxgo.CronDef[handlers.HandlerGetUser]("0 * * * *"),
        )
}
```

RouteIncome fields: `Entity`, `FromBody`, `FromQuery`, `FromParam`, `FromHeader`, `Validate`, `Cache`, `CacheTTL`, `CacheInvalidate`.

### Registration in `shared/module/main.go`

```go
// Add transport (do once per transport type)
flux.AddHttp(fluxgo.HttpOptions{Port: 3333}, func(data fluxgo.HttpConfigData) {
    data.CreateRouter("/public")
    data.CreateRouter("/api")
})
flux.AddGrpc(fluxgo.GrpcOptions{Port: 50051, Reflection: true})
flux.AddKafka(fluxgo.KafkaOptions{...})
flux.AddCron()

// Register shared deps
flux.AddDependency(repositories.UserRepositoryStart)

// Register module
flux.AddModule(user.Module())
```

### Migration (`shared/database/migrations/{version}_{name}.up.sql`)

```sql
CREATE TABLE "user" (
  id   UUID DEFAULT gen_random_uuid() PRIMARY KEY,
  name VARCHAR(255) NOT NULL
);
```

Pair with `{version}_{name}.down.sql` that reverses the change.

---

## Step 4 — Naming conventions

| What | Pattern | Example |
|------|---------|---------|
| Handler struct | `Handler{Action}{Entity}` | `HandlerGetUser` |
| Handler factory | `{HandlerName}Start` | `HandlerGetUserStart` |
| gRPC handler struct | `Handler{Entity}Grpc` | `HandlerUserGrpc` |
| Repository struct | `{Entity}Repository` | `UserRepository` |
| Repository factory | `{Entity}RepositoryStart` | `UserRepositoryStart` |
| DTO request | `{Action}Req` | `GetUserReq` |
| DTO response | `{Action}Res` | `GetUserRes` |
| Module function | `Module()` | Returns `*fluxgo.FluxModule` |
| Migration file | `{n}_{name}.up.sql` | `1_add_users.up.sql` |

---

## Step 5 — Checklist before finishing

- [ ] Entity has `TableName()` and `PrimaryKey()`
- [ ] Repository factory accepts `*fluxgo.Database`, every method calls `StartSpan`
- [ ] Handler factory registered with `AddHandler()`
- [ ] Repository factory registered with `AddDependency()` in `shared/module/main.go`
- [ ] Module registered with `AddModule()` in `shared/module/main.go`
- [ ] gRPC: proto in `proto/`, generated code in `shared/pb/`, `AddGrpc()` called
- [ ] Migration paired (.up.sql + .down.sql)
- [ ] Build passes: `go build ./...`
