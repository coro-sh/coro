package valgoutil

import (
	"testing"

	"github.com/cohesivestack/valgo"
	"github.com/stretchr/testify/assert"
)

func TestGetDetails(t *testing.T) {
	err := valgo.Is(valgo.Int(-1, "foo").EqualTo(100, "error_1").Or().InSlice([]int{100}, "error_2")).Error()
	got := GetDetails(err.(*valgo.Error))
	assert.Len(t, got, 1)

	// GetDetails iterates a map which has nondeterministic order
	wantOneOf := []string{
		"foo: [error_1, error_2]",
		"foo: [error_2, error_1]",
	}

	assert.Contains(t, wantOneOf, got[0])
}
