package fluxgo

import (
	"fmt"

	"go.uber.org/fx"
)

type FluxGo struct {
	Name     string
	Version  string
	Env      string
	Debugger bool

	dependencies []fx.Option
	invokes      []fx.Option
}

func New(config FluxGo) *FluxGo {
	init := config
	init.dependencies = []fx.Option{}

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

func (f *FluxGo) log(key, message string) {
	if f.Debugger {
		fmt.Printf("[%s]: %s\n", key, message)
	}
}
