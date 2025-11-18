package fluxgo

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

type FluxGo struct {
	Name     string
	Version  string
	Env      string
	Debugger bool

	logger *LoggerInstance
	apm    *TApm

	dependencies []fx.Option
	invokes      []fx.Option
}

func New(config FluxGo) *FluxGo {
	init := config
	init.dependencies = []fx.Option{}

	init.ConfigLogger(LoggerOptions{Type: "console"})

	return &init
}

func (f *FluxGo) AddDependency(opt fx.Option) *FluxGo {
	f.dependencies = append(f.dependencies, opt)
	f.log("DEPENDENCY/ADD", opt.String())

	return f
}
func (f *FluxGo) AddInvoke(opt fx.Option) *FluxGo {
	f.invokes = append(f.invokes, opt)
	f.log("INVOKE/ADD", opt.String())

	return f
}
func (f *FluxGo) GetFxConfig() []fx.Option {
	return append(f.dependencies, f.invokes...)
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
		fmt.Printf("[%s]: %s\n", key, message)
	}
}
