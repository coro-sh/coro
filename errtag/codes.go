package errtag

import "net/http"

type codeInternal struct{}

func (codeInternal) Code() int { return http.StatusInternalServerError }

type codeUnauthorized struct{}

func (codeUnauthorized) Code() int { return http.StatusUnauthorized }

type codeBadRequest struct{}

func (codeBadRequest) Code() int { return http.StatusBadRequest }

type codeNotFound struct{}

func (codeNotFound) Code() int { return http.StatusNotFound }

type codeConflict struct{}

func (codeConflict) Code() int { return http.StatusConflict }
