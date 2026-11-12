package handlers

import (
	"testing"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/shared/module"
	"github.com/stretchr/testify/assert"
)

func TestMultiHttpServer(t *testing.T) {
	fx, registry := fluxgo.GetTestRegistry[fluxgo.HttpRegistry](module.Module(), t)
	defer fx.RequireStart().RequireStop()

	public := registry.Get("")
	admin := registry.Get("admin")

	t.Run("should register both servers", func(t *testing.T) {
		assert.Equal(t, 2, len(registry.All()))
		assert.Equal(t, fluxgo.DefaultServerName, public.GetName())
		assert.Equal(t, "admin", admin.GetName())
		assert.Same(t, public, registry.Default())
	})

	t.Run("should serve the admin route only on the admin server", func(t *testing.T) {
		status, _ := fluxgo.RunTestRequest(admin, "GET", "/admin/user", nil, nil)
		assert.Equal(t, 200, status)

		status, _ = fluxgo.RunTestRequestRaw(public, "GET", "/admin/user", nil, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("should serve the public route only on the default server", func(t *testing.T) {
		status, _ := fluxgo.RunTestRequest(public, "GET", "/public/user", nil, nil)
		assert.Equal(t, 200, status)

		status, _ = fluxgo.RunTestRequestRaw(admin, "GET", "/public/user", nil, nil)
		assert.Equal(t, 404, status)
	})
}
