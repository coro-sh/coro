package ref

func Ptr[T any](v T) *T {
	return &v
}

func Deref[T any](ptr *T, defaultValue T) T {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}
