package static

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
	h, err := NewFileHandler(testfs, "/static/")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name             string
		path             string
		wantURL          string
		wantResponseCode int // defaults to 200
		wantETag         string
		wantCacheControl string
		wantContentType  string
	}{
		{
			name:             "file1",
			path:             "file1.txt",
			wantURL:          "/static/file1.5151b2dd.txt",
			wantETag:         "5151b2dda7951a9543b1c88d4a4a8362c22bcba91391bd79e2f2de5e2a45515b",
			wantCacheControl: "public, max-age=31536000, immutable",
			wantContentType:  "text/plain; charset=utf-8",
		},
		{
			name:             "file2",
			path:             "subdir/file2.txt",
			wantURL:          "/static/subdir/file2.687830f0.txt",
			wantETag:         "687830f0aa1e62250454259667150b0436c3bac5cbde18fea64fe078b3db5e70",
			wantCacheControl: "public, max-age=31536000, immutable",
			wantContentType:  "text/plain; charset=utf-8",
		},
		{
			name:             "js",
			path:             "test.js",
			wantURL:          "/static/test.7d9e5c06.js",
			wantETag:         "7d9e5c06589e75228ed9a85cb93074023cc796dacd7161bd014d51c88773ddc2",
			wantCacheControl: "public, max-age=31536000, immutable",
			wantContentType:  "text/javascript; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, err := h.PathFor(tt.path)
			if err != nil {
				t.Fatal(err)
			}

			if tt.wantURL != gotURL {
				t.Errorf("want URL %s, got: %s", tt.wantURL, gotURL)
			}

			req, _ := http.NewRequest("GET", gotURL, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			wantResponseCode := tt.wantResponseCode
			if wantResponseCode == 0 {
				wantResponseCode = http.StatusOK
			}
			if rr.Code != wantResponseCode {
				t.Errorf("want response code %d, got: %d", wantResponseCode, rr.Code)
			}

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

	t.Run("unversioned request", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/static/file1.txt", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("want response code %d, got: %d", http.StatusOK, rr.Code)
		}
	})
}
