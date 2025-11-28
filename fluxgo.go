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

	logger *Logger
	apm    *Apm
	http   *Http

	dependencies []fx.Option
	invokes      []fx.Option
	modules      []*FluxModule
}
type FluxGoConfig struct {
	Name         string
	Version      string
	Debugger     bool
	FullDebugger bool
	Env          *Env
}

func New(config FluxGoConfig) *FluxGo {
	init := FluxGo{
		FluxGoConfig: config,
		cleanName:    strings.ReplaceAll(strings.ToLower(config.Name), " ", "_"),
		dependencies: []fx.Option{},
		invokes:      []fx.Option{},
	}

	if init.Env == nil {
		env := ParseEnv[Env](EnvOptions{})
		init.Env = &env
	}

	init.ConfigLogger(LoggerOptions{Type: "console"})

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

func (f *FluxGo) AddModule(mod *FluxModule) *FluxGo {
	f.modules = append(f.modules, mod)

	return f
}

func (f *FluxGo) GetFxConfig() []fx.Option {
	f.AddDependency(func() *Logger { return f.logger })

	f.dependencies = append(f.dependencies, fx.Provide(func() *FluxGo {
		return f
	}))

	full := append(f.dependencies, f.invokes...)
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

func (f *FluxGo) log(key, message string) {
	if f.Debugger {
		fmt.Printf("%s [%s]: %s\n", time.Now().Format(time.DateTime), key, message)
	}
}
