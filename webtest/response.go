package webtest

import (
	"net/http"
	"net/http/httptest"

	"lds.li/web"
)

type Response struct {
	web.ResponseWriter
	recorder *httptest.ResponseRecorder
}

func (r *Response) Result() *http.Response {
	return r.recorder.Result()
}

func NewResponse() *Response {
	rec := httptest.NewRecorder()
	return &Response{ResponseWriter: web.NewResponseWriter(rec), recorder: rec}
}
