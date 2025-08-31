package webtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http/httptest"

	"github.com/lstoll/web"
	"github.com/lstoll/web/internal/ctxkeys"
	"github.com/lstoll/web/session"
	"github.com/lstoll/web/static"
)

type requestOpts struct {
	sessionValues map[string]any
	jsonBody      any
	staticContent fs.FS
	staticPrefix  string
}

type RequestOpt func(opts *requestOpts) error

func RequestWithSessionValues(values map[string]any) RequestOpt {
	return func(opts *requestOpts) error {
		opts.sessionValues = values
		return nil
	}
}

func RequestWithJSONBody(body any) RequestOpt {
	return func(opts *requestOpts) error {
		opts.jsonBody = body
		return nil
	}
}

func RequestWithStaticContent(fs fs.FS, prefix string) RequestOpt {
	return func(opts *requestOpts) error {
		opts.staticContent = fs
		opts.staticPrefix = prefix
		return nil
	}
}

func NewRequest(method string, url string, opts ...RequestOpt) *web.Request {
	ropts := &requestOpts{}
	for _, opt := range opts {
		if err := opt(ropts); err != nil {
			panic(fmt.Errorf("applying request opt: %w", err))
		}
	}

	var body io.Reader
	if ropts.jsonBody != nil {
		b, err := json.Marshal(ropts.jsonBody)
		if err != nil {
			panic(fmt.Errorf("marshalling json body: %w", err))
		}
		body = bytes.NewReader(b)
	}

	r := httptest.NewRequest(method, url, body)

	if ropts.jsonBody != nil {
		r.Header.Set("Content-Type", "application/json")
	}

	if ropts.staticContent != nil {
		sh, err := static.NewFileHandler(ropts.staticContent, ropts.staticPrefix)
		if err != nil {
			panic(fmt.Errorf("creating static handler: %w", err))
		}
		r = r.WithContext(ctxkeys.ContextWithStaticHandler(r.Context(), sh))
	}

	ctx, _ := session.TestContext(r.Context(), nil)

	wr := web.NewRequestFrom(r.WithContext(ctx))

	if ropts.sessionValues != nil {
		for k, v := range ropts.sessionValues {
			wr.Session().Set(k, v)
		}
	}

	return wr
}
