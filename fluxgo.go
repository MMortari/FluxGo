package fluxgo

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type FluxGo struct {
	FluxGoConfig
	cleanName string

	otel *Otel
	db   *Database

	dependencies []fx.Option
	invokes      []fx.Option
	replaces     []fx.Option
	supplies     []fx.Option
	modules      []*FluxModule
}
type FluxGoConfig struct {
	Name         string
	Version      string
	Debugger     bool
	FullDebugger bool
	Env          *Env
	Otel         *OtelOptions
}

func New(config FluxGoConfig) *FluxGo {
	init := FluxGo{
		FluxGoConfig: config,
		cleanName:    strings.ReplaceAll(strings.ToLower(config.Name), " ", "_"),
		dependencies: []fx.Option{},
		invokes:      []fx.Option{},
		replaces:     []fx.Option{},
		supplies:     []fx.Option{},
	}

	if init.Env == nil {
		env := ParseEnv[Env](EnvOptions{})
		init.Env = &env
	}

	init.dependencies = append(init.dependencies, fx.Provide(func() *FluxGo { return &init }))

	init.db = &Database{dbs: make(map[string]*databaseData)}
	init.dependencies = append(init.dependencies, fx.Provide(func() *Database { return init.db }))

	if init.Otel != nil {
		init.addOtel(*init.Otel)
	} else {
		init.addOtel(OtelOptions{Exporter: "log"})
	}

	return &init
}

func (f *FluxGo) GetName() string {
	return f.Name
}
func (f *FluxGo) GetCleanName() string {
	return f.cleanName
}

func (f *FluxGo) AddDependency(constructors ...interface{}) *FluxGo {
	opt := fx.Provide(constructors...)
	f.dependencies = append(f.dependencies, opt)

	return f
}
func (f *FluxGo) AddInvoke(constructors ...interface{}) *FluxGo {
	opt := fx.Invoke(constructors...)
	f.invokes = append(f.invokes, opt)

	return f
}
func (f *FluxGo) AddReplace(constructors ...interface{}) *FluxGo {
	opt := fx.Replace(constructors...)
	f.replaces = append(f.replaces, opt)

	return f
}
func (f *FluxGo) AddSupply(constructors ...interface{}) *FluxGo {
	opt := fx.Supply(constructors...)
	f.supplies = append(f.supplies, opt)

	return f
}

func (f *FluxGo) AddModule(mod *FluxModule) *FluxGo {
	f.modules = append(f.modules, mod)

	return f
}

func (f *FluxGo) GetFxConfig() []fx.Option {
	full := append(f.dependencies, f.invokes...)
	full = append(full, f.replaces...)
	full = append(full, f.supplies...)

	modules := []fx.Option{}
	for _, module := range f.modules {
		modules = append(modules, module.toFx())
	}
	full = append(full, modules...)

	if !f.FullDebugger {
		full = append(full, fx.NopLogger)
	}

	return full
}

func (f *FluxGo) Run() {
	fx.New(f.GetFxConfig()...).Run()
}

func (f *FluxGo) GetTestApp(t *testing.T) (*fxtest.App, *Http) {
	f.Debugger = false
	f.FullDebugger = false

	var http *Http

	opts := append(f.GetFxConfig(), fx.Invoke(func(h *Http) {
		http = h
	}), fx.NopLogger)

	fxApp := fxtest.New(t, opts...)

	return fxApp, http
}

func (f *FluxGo) Log(key, message string) {
	if f.Debugger {
		fmt.Printf("%s [%s] %s\n", time.Now().Format(time.DateTime), key, message)
	}
}
