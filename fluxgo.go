package fluxgo

import (
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type FluxGo struct {
	Name         string
	Version      string
	Debugger     bool
	FullDebugger bool

	logger *Logger
	apm    *Apm
	http   *Http
	Env    *Env

	dependencies []fx.Option
	invokes      []fx.Option
	modules      []*FluxModule
}

func New(config FluxGo) *FluxGo {
	init := config
	init.dependencies = []fx.Option{}

	init.ConfigLogger(LoggerOptions{Type: "console"})

	return &init
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

func (f *FluxGo) GetTestApp(t *testing.T) (*fxtest.App, *fiber.App) {
	var app *fiber.App

	opts := append(f.GetFxConfig(), fx.Invoke(func(a *fiber.App) {
		app = a
	}), fx.NopLogger)

	fxApp := fxtest.New(t, opts...)

	return fxApp, app
}

func (f *FluxGo) log(key, message string) {
	if f.Debugger {
		fmt.Printf("%s [%s]: %s\n", time.Now().Format(time.DateTime), key, message)
	}
}
