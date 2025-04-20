package session

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCookieManager_RoundTrip(t *testing.T) {
	// Create a new AEAD for testing
	aead, err := NewXChaPolyAEAD(genXChaPolyKey(), nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name                 string
		data                 []byte
		expiresAt            time.Time
		compressionDisabled  bool
		expectCompression    bool
		expectSaveError      bool
		expectRoundTripError bool
	}{
		{
			name:              "Small data, no compression",
			data:              []byte("small test data"),
			expiresAt:         time.Now().Add(1 * time.Hour),
			expectCompression: false,
		},
		{
			name:              "Large data, with compression",
			data:              randBytes(managerMaxCookieSize - 1500),
			expiresAt:         time.Now().Add(1 * time.Hour),
			expectCompression: true,
		},
		{
			name:                 "Expired data",
			data:                 []byte("test data"),
			expiresAt:            time.Now().Add(-1 * time.Hour),
			expectRoundTripError: true,
		},
		{
			name:                "Large data, compression disabled",
			data:                randBytes(managerMaxCookieSize - 1500),
			expiresAt:           time.Now().Add(1 * time.Hour),
			compressionDisabled: true,
			expectCompression:   false,
		},
		{
			name:              "Data just below compression threshold",
			data:              bytes.Repeat([]byte("a"), managerCompressThreshold-9),
			expiresAt:         time.Now().Add(1 * time.Hour),
			expectCompression: false,
		},
		{
			name:              "Empty data",
			data:              []byte{},
			expiresAt:         time.Now().Add(1 * time.Hour),
			expectCompression: false,
		},
		{
			name:              "Binary data with zero bytes",
			data:              []byte{0, 1, 2, 0, 3, 4},
			expiresAt:         time.Now().Add(1 * time.Hour),
			expectCompression: false,
		},
		{
			name:              "Almost expiring data",
			data:              []byte("test data"),
			expiresAt:         time.Now().Add(1 * time.Second),
			expectCompression: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a manager for this test
			mgr, err := NewCookieManager(aead, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Set compression flag
			mgr.compressionDisabled = tt.compressionDisabled

			// Create request and response recorders for testing
			w := httptest.NewRecorder()

			// Save cookie
			err = mgr.saveToCookie(w, nil, tt.expiresAt, tt.data)
			if tt.expectSaveError {
				if err == nil {
					t.Error("Expected save error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error saving cookie: %v", err)
			}

			// Get the cookie from the response
			cookies := w.Result().Cookies()
			if len(cookies) == 0 {
				t.Fatal("No cookies set in response")
			}

			// Verify the cookie value has the expected magic prefix
			cookieValue := cookies[0].Value
			parts := strings.SplitN(cookieValue, ".", 2)
			if len(parts) != 2 {
				t.Fatalf("Invalid cookie format: %s", cookieValue)
			}

			actualMagic := parts[0]
			expectedMagic := managerCookieMagic
			if tt.expectCompression {
				expectedMagic = managerCompressedCookieMagic
			}

			if actualMagic != expectedMagic {
				t.Errorf("Expected cookie magic %s, got %s", expectedMagic, actualMagic)
			}

			// Load the cookie back
			loadedData, err := mgr.loadFromCookie(cookieValue)

			if tt.expectRoundTripError {
				if err == nil {
					t.Error("Expected load error due to expiration, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error loading cookie: %v", err)
			}

			// Verify that the data matches what we saved
			if !bytes.Equal(loadedData, tt.data) {
				t.Errorf("Data mismatch after round trip:\nExpected: %v\nGot: %v", tt.data, loadedData)
			}
		})
	}
}

// TestCookieManager_ExtremelyLargeData tests that very large data causes an error
func TestCookieManager_ExtremelyLargeData(t *testing.T) {
	// Create a new AEAD for testing
	aead, err := NewXChaPolyAEAD(genXChaPolyKey(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a manager
	mgr, err := NewCookieManager(aead, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create extremely large test data - large enough to exceed cookie size after encryption and encoding
	largeData := randBytes(managerMaxCookieSize)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Attempt to save
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	err = mgr.saveToCookie(w, r, expiresAt, largeData)
	if err == nil {
		// If no error, verify that the cookie size is actually large
		cookies := w.Result().Cookies()
		if len(cookies) > 0 {
			if len(cookies[0].Value) <= managerMaxCookieSize {
				t.Logf("Cookie size: %d, max allowed: %d", len(cookies[0].Value), managerMaxCookieSize)
				t.Errorf("Generated cookie is smaller than the max size (%d <= %d), adjust test data size",
					len(cookies[0].Value), managerMaxCookieSize)
			} else {
				t.Errorf("Cookie size %d exceeds max %d but no error was returned",
					len(cookies[0].Value), managerMaxCookieSize)
			}
		}
	}
}

// TestCookieManager_MultipleRoundTrips tests that data can be saved and loaded multiple times
func TestCookieManager_MultipleRoundTrips(t *testing.T) {
	// Create a new AEAD for testing
	aead, err := NewXChaPolyAEAD(genXChaPolyKey(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a manager
	mgr, err := NewCookieManager(aead, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create test data
	originalData := []byte("test data for multiple round trips")
	expiresAt := time.Now().Add(1 * time.Hour)

	// Round trip 1
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/", nil)

	err = mgr.saveToCookie(w1, r1, expiresAt, originalData)
	if err != nil {
		t.Fatalf("Error in first save: %v", err)
	}

	cookies1 := w1.Result().Cookies()
	loadedData1, err := mgr.loadFromCookie(cookies1[0].Value)
	if err != nil {
		t.Fatalf("Error in first load: %v", err)
	}

	if !bytes.Equal(loadedData1, originalData) {
		t.Errorf("Data mismatch after first round trip")
	}

	// Round trip 2 - using the loaded data as input
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)

	err = mgr.saveToCookie(w2, r2, expiresAt, loadedData1)
	if err != nil {
		t.Fatalf("Error in second save: %v", err)
	}

	cookies2 := w2.Result().Cookies()
	loadedData2, err := mgr.loadFromCookie(cookies2[0].Value)
	if err != nil {
		t.Fatalf("Error in second load: %v", err)
	}

	if !bytes.Equal(loadedData2, originalData) {
		t.Errorf("Data mismatch after second round trip")
	}
}

// TestCookieManager_CompressionLogic tests the compression logic specifically
func TestCookieManager_CompressionLogic(t *testing.T) {
	// Create a new AEAD for testing
	aead, err := NewXChaPolyAEAD(genXChaPolyKey(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a manager
	mgr, err := NewCookieManager(aead, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Test with data size that should trigger compression
	// The 8 bytes for expiry time gets added to this
	largeData := bytes.Repeat([]byte("a"), managerCompressThreshold+1)
	expiresAt := time.Now().Add(1 * time.Hour)

	// Save to cookie
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	err = mgr.saveToCookie(w, r, expiresAt, largeData)
	if err != nil {
		t.Fatalf("Error saving cookie: %v", err)
	}

	// Get cookie value and check the magic
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("No cookies set")
	}

	cookieValue := cookies[0].Value
	t.Logf("Cookie value: %s", cookieValue)

	parts := strings.SplitN(cookieValue, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("Invalid cookie format: %s", cookieValue)
	}

	magic := parts[0]
	t.Logf("Magic: %s, Expected EC1 (compressed)", magic)

	if magic != managerCompressedCookieMagic {
		t.Errorf("Expected cookie magic %s for compression, got %s",
			managerCompressedCookieMagic, magic)
	}

	// Now try to load it back
	loadedData, err := mgr.loadFromCookie(cookieValue)
	if err != nil {
		t.Fatalf("Error loading cookie: %v", err)
	}

	// Verify data round-tripped correctly
	if !bytes.Equal(loadedData, largeData) {
		t.Error("Data mismatch after compression round-trip")
	}
}

// TestCookieManager_MaxSize tests the cookie max size limit
func TestCookieManager_MaxSize(t *testing.T) {
	// Create a new AEAD for testing
	aead, err := NewXChaPolyAEAD(genXChaPolyKey(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a manager
	mgr, err := NewCookieManager(aead, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Try increasingly large data sizes until we hit the limit
	sizes := []int{1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000}
	expiresAt := time.Now().Add(1 * time.Hour)

	for _, size := range sizes {
		t.Run(fmt.Sprintf("Size_%d", size), func(t *testing.T) {
			data := randBytes(size)

			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			err = mgr.saveToCookie(w, r, expiresAt, data)

			if err != nil {
				if strings.Contains(err.Error(), "cookie size") {
					t.Logf("Size %d exceeded cookie size limit as expected", size)
				} else {
					t.Errorf("Unexpected error for size %d: %v", size, err)
				}
				return
			}

			// If no error, check cookie size
			cookies := w.Result().Cookies()
			if len(cookies) > 0 {
				cookieSize := len(cookies[0].Value)
				t.Logf("Cookie size for data size %d: %d bytes (max: %d)",
					size, cookieSize, managerMaxCookieSize)

				if cookieSize > managerMaxCookieSize {
					t.Errorf("Cookie size %d exceeds max %d but no error",
						cookieSize, managerMaxCookieSize)
				}
			}
		})
	}
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}
