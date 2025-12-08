package fluxgo

import (
	"math/rand"
	"runtime"
	"strings"
)

func Pointer[T any](val T) *T {
	return &val
}

func Default[T any](val *T, defaultVal T) T {
	if val != nil {
		return *val
	}
	return defaultVal
}

func FunctionCaller(skip int) string {
	pc, _, _, _ := runtime.Caller(skip)
	caller := runtime.FuncForPC(pc)

	splitted := strings.Split((caller.Name()), ".")

	return splitted[len(splitted)-1]
}

func GetRandomNumber(max int) int {
	return rand.Intn(max + 1)
}
