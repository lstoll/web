package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

type staticFileHandler struct {
	fs        fs.FS
	checksums map[string]string
	prefix    string
	mtime     time.Time
}

func newStaticFileHandler(f fs.FS, prefix string) (*staticFileHandler, error) {
	h := &staticFileHandler{
		fs:        f,
		checksums: make(map[string]string),
		prefix:    prefix,
	}

	mt, err := embedModTime()
	if err != nil {
		return nil, err
	}
	h.mtime = mt

	if err := fs.WalkDir(f, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		f, err := f.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, ok := f.(io.ReadSeeker); !ok {
			return fmt.Errorf("file %s is not an io.ReadSeeker", path)
		}

		hasher := sha256.New()
		if _, err := io.Copy(hasher, f); err != nil {
			return err
		}
		h.checksums[path] = hex.EncodeToString(hasher.Sum(nil))[0:16]
		return nil
	}); err != nil {
		return nil, err
	}

	return h, nil
}

func (h *staticFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, h.prefix)
	if p == "" || p == "/" {
		http.NotFound(w, r)
		return
	}

	f, err := h.fs.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	etag := h.checksums[p]
	if etag == "" {
		http.NotFound(w, r)
		return
	}

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", etag)

	if _, ok := r.URL.Query()["sum"]; ok {
		w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeContent(w, r, path.Base(p), h.mtime, f.(io.ReadSeeker))
}

func (h *staticFileHandler) URL(path string) (string, error) {
	checksum, ok := h.checksums[path]
	if !ok {
		return "", fmt.Errorf("file %s does not exist", path)
	}
	u := url.URL{Path: h.prefix + path}
	q := u.Query()
	q.Set("sum", checksum)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func embedModTime() (time.Time, error) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return time.Time{}, errors.New("failed to read build info")
	}

	var (
		bt       time.Time
		modified bool
	)
	for _, bs := range bi.Settings {
		if bs.Key == "vcs.time" {
			bt, _ = time.Parse(time.RFC3339, bs.Value)
		}
		if bs.Key == "vcs.modified" {
			modified, _ = strconv.ParseBool(bs.Value)
		}
	}
	if modified || bt.IsZero() {
		return time.Now(), nil
	}

	return bt, nil
}
