package web

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed testdata
var testdata embed.FS

var testfs = func() fs.FS {
	sfs, err := fs.Sub(testdata, "testdata")
	if err != nil {
		panic(err)
	}
	return sfs
}()

func TestStaticFileHandler(t *testing.T) {
	h, err := newStaticFileHandler(testfs, "/static/")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name             string
		path             string
		wantURL          string
		wantETag         string
		wantCacheControl string
		wantContentType  string
	}{
		{
			name:             "file1",
			path:             "file1.txt",
			wantURL:          "/static/file1.txt?sum=5151b2dda7951a95",
			wantETag:         "5151b2dda7951a95",
			wantCacheControl: "public, max-age=31536000",
			wantContentType:  "text/plain; charset=utf-8",
		},
		{
			name:             "file2",
			path:             "subdir/file2.txt",
			wantURL:          "/static/subdir/file2.txt?sum=687830f0aa1e6225",
			wantETag:         "687830f0aa1e6225",
			wantCacheControl: "public, max-age=31536000",
			wantContentType:  "text/plain; charset=utf-8",
		},
		{
			name:             "js",
			path:             "test.js",
			wantURL:          "/static/test.js?sum=7d9e5c06589e7522",
			wantETag:         "7d9e5c06589e7522",
			wantCacheControl: "public, max-age=31536000",
			wantContentType:  "text/javascript; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, err := h.URL(tt.path)
			if err != nil {
				t.Fatal(err)
			}

			if tt.wantURL != gotURL {
				t.Errorf("want URL %s, got: %s", tt.wantURL, gotURL)
			}

			req, _ := http.NewRequest("GET", gotURL, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if tt.wantETag != rr.Header().Get("ETag") {
				t.Errorf("want etag %s, got: %s", tt.wantETag, rr.Header().Get("ETag"))
			}

			if tt.wantCacheControl != rr.Header().Get("Cache-Control") {
				t.Errorf("want cache-control %s, got: %s", tt.wantCacheControl, rr.Header().Get("Cache-Control"))
			}

			if tt.wantContentType != rr.Header().Get("Content-Type") {
				t.Errorf("want content-type %s, got: %s", tt.wantContentType, rr.Header().Get("Content-Type"))
			}
		})
	}

	req, _ := http.NewRequest("GET", "/static/file1.txt", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("want cache-control %s, got: %s", "no-store", rr.Header().Get("Cache-Control"))
	}
}
