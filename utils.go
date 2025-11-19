package fluxgo

func Pointer[T any](val T) *T {
	return &val
}

func Default[T any](val *T, defaultVal T) T {
	if val != nil {
		return *val
	}
	return defaultVal
}
