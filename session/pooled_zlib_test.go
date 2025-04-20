package session

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"strings"
	"sync"
	"testing"
)

func TestCompression(t *testing.T) {
	var (
		wg   sync.WaitGroup
		errC = make(chan (error), 1000)
	)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			data := randStr(4096)

			for range 20 {
				cw := getCompressor()
				cr := getDecompressor()

				b, err := cw.Compress([]byte(data))
				if err != nil {
					errC <- fmt.Errorf("compressing data: %v", err)
					return
				}

				dec, err := cr.Decompress(b)
				if err != nil {
					errC <- fmt.Errorf("decompressing data: %v", err)
					return
				}

				if !strings.EqualFold(data, string(dec)) {
					errC <- errors.New("decompressed data does not match compressed")
					return
				}

				putCompressor(cw)
				putDecompressor(cr)
			}
		}()
	}
	wg.Wait()

	select {
	case err := <-errC:
		t.Fatalf("at least one error occured, first error: %v", err)
	default:
		// pass
	}
}

func BenchmarkCompression(b *testing.B) {
	b.Run("unpooled", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			data := randStr(4096)
			b.StartTimer()

			var buf bytes.Buffer
			wr := zlib.NewWriter(&buf)
			if _, err := wr.Write([]byte(data)); err != nil {
				b.Fatal(err)
			}
			if err := wr.Close(); err != nil {
				b.Fatal(err)
			}

			r, err := zlib.NewReader(&buf)
			if err != nil {
				b.Fatal(err)
			}
			rb, err := io.ReadAll(r)
			if err != nil {
				b.Fatal(err)
			}

			b.StopTimer()
			if !strings.EqualFold(data, string(rb)) {
				b.Fatal("data does not match")
			}

			b.SetBytes(int64(len(data)))
		}
	})

	b.Run("pooled", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			data := randStr(4096)
			b.StartTimer()

			cw := getCompressor()
			cb, err := cw.Compress([]byte(data))
			if err != nil {
				b.Fatal(err)
			}
			putCompressor(cw)

			cr := getDecompressor()
			rb, err := cr.Decompress(cb)
			if err != nil {
				b.Fatal(err)
			}
			putDecompressor(cr)

			b.StopTimer()
			if !strings.EqualFold(data, string(rb)) {
				b.Fatal("data does not match")
			}

			b.SetBytes(int64(len(data)))
		}
	})
}

var randChars = []rune(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890`)

func randStr(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = randChars[mathrand.IntN(len(randChars))]
	}
	return string(b)
}
