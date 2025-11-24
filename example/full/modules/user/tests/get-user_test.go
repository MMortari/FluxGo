package handlers

import (
	"testing"

	fluxgo "github.com/MMortari/FluxGo"
	"github.com/MMortari/FluxGo/example/full/shared/module"
	"github.com/stretchr/testify/assert"
)

func TestOnboardContact(t *testing.T) {
	fx, app := module.Module().GetTestApp(t)
	defer fx.RequireStart().RequireStop()

	endpoint := "/public/user"
	successHttpCode := 200

	t.Run("Success", func(t *testing.T) {
		status, body := fluxgo.RunTestRequest(app, "GET", endpoint, nil, nil)

		assert.Equalf(t, successHttpCode, status, "Invalid status code")
		assert.NotNilf(t, body["user"], "Invalid body response")

		user := fluxgo.ConvertToMap(body["user"])
		assert.Equalf(t, "299f3dcd-42f3-46c1-89d5-603c78a78f50", user["id"], "Invalid body response")
		assert.Equalf(t, "John", user["name"], "Invalid body response")
	})
}
