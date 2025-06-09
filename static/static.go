package static

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const sumLength = 8

type FileHandler struct {
	fs        fs.FS
	checksums map[string]string
	prefix    string
	mtime     time.Time
}

func NewFileHandler(f fs.FS, prefix string) (*FileHandler, error) {
	h := &FileHandler{
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
		h.checksums[path] = hex.EncodeToString(hasher.Sum(nil))
		return nil
	}); err != nil {
		return nil, err
	}

	return h, nil
}

func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, h.prefix)
	if p == "" || p == "/" {
		http.NotFound(w, r)
		return
	}

	// Check if this is a versioned file (contains checksum in filename)
	originalPath, checksum, isVersioned := h.parseVersionedPath(p)

	var filePath string
	var expectedChecksum string
	var useMaxAge bool

	if isVersioned {
		// For versioned files, use the original path and validate checksum
		filePath = originalPath
		expectedChecksum = h.checksums[originalPath]
		if expectedChecksum == "" || expectedChecksum[0:sumLength] != checksum {
			http.NotFound(w, r)
			return
		}
		useMaxAge = true
	} else {
		// For non-versioned files, use the path directly
		filePath = p
		expectedChecksum = h.checksums[p]
		if expectedChecksum == "" {
			http.NotFound(w, r)
			return
		}
		useMaxAge = false
	}

	f, err := h.fs.Open(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	if r.Header.Get("If-None-Match") == expectedChecksum {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", expectedChecksum)

	if useMaxAge {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // 1 year
	}

	http.ServeContent(w, r, path.Base(filePath), h.mtime, f.(io.ReadSeeker))
}

func (h *FileHandler) PathFor(filePath string) (string, error) {
	checksum, ok := h.checksums[filePath]
	if !ok {
		return "", fmt.Errorf("file %s does not exist", filePath)
	}

	// Use truncated checksum for filename (for shorter URLs)
	shortChecksum := checksum[0:sumLength]

	// Insert checksum into filename
	dir := path.Dir(filePath)
	filename := path.Base(filePath)

	// Find the last dot for the file extension
	lastDot := strings.LastIndex(filename, ".")
	var versionedFilename string

	if lastDot == -1 {
		// No extension, append checksum
		versionedFilename = filename + "." + shortChecksum
	} else {
		// Insert checksum before extension
		versionedFilename = filename[:lastDot] + "." + shortChecksum + filename[lastDot:]
	}

	versionedPath := path.Join(dir, versionedFilename)
	return h.prefix + versionedPath, nil
}

func (h *FileHandler) parseVersionedPath(p string) (originalPath string, checksum string, isVersioned bool) {
	dir := path.Dir(p)
	filename := path.Base(p)

	// Find the last dot (file extension)
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		return p, "", false
	}

	// Find the second-to-last dot (checksum separator)
	beforeExt := filename[:lastDot]
	secondLastDot := strings.LastIndex(beforeExt, ".")
	if secondLastDot == -1 {
		return p, "", false
	}

	// Extract potential checksum
	checksum = beforeExt[secondLastDot+1:]
	if len(checksum) != sumLength {
		return p, "", false
	}

	// Reconstruct original filename
	originalFilename := beforeExt[:secondLastDot] + filename[lastDot:]
	originalPath = path.Join(dir, originalFilename)

	return originalPath, checksum, true
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
