package errtag

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMsg(t *testing.T) {
	var meta tagMeta
	opt := WithMsg("custom message")
	opt(&meta)

	assert.Equal(t, "custom message", meta.msg)
}

func TestWithMsgf(t *testing.T) {
	var meta tagMeta
	opt := WithMsgf("formatted %s", "message")
	opt(&meta)

	assert.Equal(t, "formatted message", meta.msg)
}

func TestWithDetails(t *testing.T) {
	var meta tagMeta
	opt := WithDetails("detail1", "detail2")
	opt(&meta)

	assert.Equal(t, []string{"detail1", "detail2"}, meta.details)
}

func TestTag(t *testing.T) {
	err := errors.New("cause error")
	tag := Tag[NotFound, *NotFound](err, WithMsg("not found"), WithDetails("detail"))

	require.NotNil(t, tag)
	assert.Equal(t, http.StatusNotFound, tag.Code())
	assert.Equal(t, "not found", tag.Msg())
	assert.Equal(t, "cause error", tag.Error())
	assert.Equal(t, []string{"detail"}, tag.Details())
}

func TestNewTagged(t *testing.T) {
	taggedErr := NewTagged[Unauthorized, *Unauthorized]("unauthorized access", WithMsg("unauthorized"))
	require.NotNil(t, taggedErr)

	asUnauthorized, ok := AsTag[Unauthorized](taggedErr)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, asUnauthorized.Code())
	assert.Equal(t, "unauthorized", asUnauthorized.Msg())
	assert.Equal(t, "unauthorized access", asUnauthorized.Error())
}
