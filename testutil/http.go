package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

var DefaultClient = &http.Client{
	Transport: &http.Transport{MaxIdleConnsPerHost: 1},
	Timeout:   10 * time.Second,
}

func Get[R any](t *testing.T, url string) R {
	t.Helper()
	httpRes, err := DefaultClient.Get(url)
	require.NoError(t, err)
	defer httpRes.Body.Close()

	handleHTTPErr(t, httpRes, http.MethodGet, url)

	var res R
	err = json.NewDecoder(httpRes.Body).Decode(&res)
	require.NoError(t, err)
	return res
}

func GetText(t *testing.T, url string) string {
	t.Helper()
	httpRes, err := DefaultClient.Get(url)
	require.NoError(t, err)
	defer httpRes.Body.Close()

	handleHTTPErr(t, httpRes, http.MethodGet, url)

	body, err := io.ReadAll(httpRes.Body)
	require.NoError(t, err)
	return string(body)
}

func Post[R any](t *testing.T, url string, req any) R {
	t.Helper()
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	httpRes, err := DefaultClient.Post(url, echo.MIMEApplicationJSON, bytes.NewReader(reqJSON))
	require.NoError(t, err)
	defer httpRes.Body.Close()

	handleHTTPErr(t, httpRes, http.MethodPost, url)

	var res R
	err = json.NewDecoder(httpRes.Body).Decode(&res)
	require.NoError(t, err)
	return res
}

func Put[R any](t *testing.T, url string, req any) R {
	t.Helper()
	reqJSON, err := json.Marshal(req)
	require.NoError(t, err)

	putReq, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(reqJSON))
	require.NoError(t, err)
	putReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	httpRes, err := DefaultClient.Do(putReq)
	require.NoError(t, err)
	defer httpRes.Body.Close()

	handleHTTPErr(t, httpRes, http.MethodPut, url)

	var res R
	err = json.NewDecoder(httpRes.Body).Decode(&res)
	require.NoError(t, err)
	return res
}

func Delete(t *testing.T, url string) {
	t.Helper()
	deleteReq, err := http.NewRequest(http.MethodDelete, url, nil)
	require.NoError(t, err)

	httpRes, err := DefaultClient.Do(deleteReq)
	require.NoError(t, err)
	defer httpRes.Body.Close()

	handleHTTPErr(t, httpRes, http.MethodDelete, url)
}

func handleHTTPErr(t *testing.T, httpRes *http.Response, method string, url string) {
	t.Helper()
	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		body, _ := io.ReadAll(httpRes.Body)
		require.Failf(t, "http error", "%s %s\nStatus: %s\nBody: %s", method, url, httpRes.Status, string(body))
	}
}
