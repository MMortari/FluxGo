package fluxgo

import (
	"testing"

	"go.uber.org/fx"
)

func TestMain(t *testing.T) {
	t.Run("TestDependenciesAreSatisfied", func(t *testing.T) {
		flux := New(FluxGoConfig{Name: "Test", Debugger: true})

		if err := fx.ValidateApp(flux.GetFxConfig()...); err != nil {
			t.Error(err)
		}
	})
}
