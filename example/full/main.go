package main

import (
	"github.com/MMortari/FluxGo/example/full/shared/module"
)

func main() {
	flux := module.Module()

	flux.Run()
}
