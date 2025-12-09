package fluxgo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUtil(t *testing.T) {
	t.Run("Pointer", func(t *testing.T) {
		assert.Equalf(t, 5, *Pointer(5), "Invalid status code")
		assert.Equalf(t, "*int", reflect.TypeOf(Pointer(5)).String(), "Invalid status code")
		assert.Equalf(t, "ptr", reflect.TypeOf(Pointer(5)).Kind().String(), "Invalid status code")
	})
	t.Run("Default", func(t *testing.T) {
		assert.Equalf(t, 5, Default(Pointer(5), 10), "Invalid status code")
		assert.Equalf(t, 10, Default(nil, 10), "Invalid status code")
	})
	t.Run("GetRandomNumber", func(t *testing.T) {
		max := 10

		for i := 0; i < 100; i++ {
			val := GetRandomNumber(max)
			println(val)
			assert.Truef(t, val >= 0 && val < max, "Invalid number")
		}
	})
}
