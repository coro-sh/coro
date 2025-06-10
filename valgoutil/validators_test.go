package valgoutil

import (
	"testing"

	"github.com/cohesivestack/valgo"
	"github.com/stretchr/testify/assert"
)

func TestHostPortValidator(t *testing.T) {
	ok := valgo.Is(HostPortValidator("invalid", "foo")).Valid()
	assert.False(t, ok)
}

func TestNonEmptySliceValidator(t *testing.T) {
	ok := valgo.Is(NonEmptySliceValidator[string]([]string{}, "foo")).Valid()
	assert.False(t, ok)
}

func TestURLValidator(t *testing.T) {
	ok := valgo.Is(URLValidator("invalid.com", "foo")).Valid()
	assert.False(t, ok)
}
