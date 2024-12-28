package server

import (
	"fmt"
	"strings"
)

type Response[T any] struct {
	Data T `json:"data"`
}

type ResponseList[T any] struct {
	Data           []T     `json:"data"`
	NextPageCursor *string `json:"next_page_cursor,omitempty"`
}

type ResponseError struct {
	Error HTTPError `json:"error"`
}

type HTTPError struct {
	Code     int      `json:"-"`
	Internal string   `json:"-"`
	Message  string   `json:"message"`
	Details  []string `json:"details,omitempty"`
}

func (e HTTPError) Error() string {
	if len(e.Details) > 0 {
		return fmt.Sprintf("%s: %s", e.Message, strings.Join(e.Details, "; "))
	}
	return e.Message
}

type HealthResponse struct {
	Status string `json:"status"`
}
