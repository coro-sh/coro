package errtag

import (
	"errors"
	"fmt"
	"net/http"
)

type Option func(m *tagMeta)

func WithMsg(message string) Option {
	return func(t *tagMeta) {
		t.msg = message
	}
}
func WithMsgf(format string, a ...any) Option {
	return func(t *tagMeta) {
		t.msg = fmt.Sprintf(format, a...)
	}
}

func WithDetails(details ...string) Option {
	return func(t *tagMeta) {
		t.details = details
	}
}

type Tagger interface {
	error
	Code() int
	Msg() string
	Details() []string
}

type TaggerPtr[T any] interface {
	*T
	init(cause error, opts ...Option)
}

func Tag[T Tagger, TP TaggerPtr[T]](cause error, opts ...Option) T {
	var t T
	TP(&t).init(cause, opts...)
	return t
}

func NewTagged[T Tagger, TP TaggerPtr[T]](cause string, opts ...Option) error {
	var t T
	TP(&t).init(errors.New(cause), opts...)
	return t
}

type Coder interface {
	Code() int
}

type ErrorTag[C Coder] struct {
	tagMeta
}

type tagMeta struct {
	cause   error
	msg     string
	details []string
}

func (t ErrorTag[C]) Error() string {
	if t.cause == nil {
		return t.Msg()
	}
	return t.cause.Error()
}

func (t ErrorTag[C]) Cause() error {
	return t.cause
}

func (t ErrorTag[C]) Unwrap() error {
	return t.cause
}

func (t ErrorTag[C]) Code() int {
	var c C
	return c.Code()
}

func (t ErrorTag[C]) Msg() string {
	if t.msg == "" {
		return http.StatusText(t.Code())
	}
	return t.msg
}

func (t ErrorTag[C]) Details() []string {
	return t.details
}

func (t *ErrorTag[C]) init(cause error, opts ...Option) {
	t.tagMeta.cause = cause
	for _, opt := range opts {
		opt(&t.tagMeta)
	}
}

func HasTag[T Tagger](err error) bool {
	_, ok := AsTag[T](err)
	return ok
}

func AsTag[T Tagger](err error) (T, bool) {
	var out T
	if err == nil {
		return out, false
	}
	ok := errors.As(err, &out)
	return out, ok
}
