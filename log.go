package web

import (
	"log/slog"
	"net/http"
	"time"
)

// loggingResponseWriter wraps the standard http.ResponseWriter to capture status and bytes written.
type loggingResponseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytesWritten += n
	return n, err
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lrw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		status := lrw.status
		if status == 0 {
			status = http.StatusOK
		}

		slog.InfoContext(r.Context(), "Request Served",
			slog.String("remote_addr", r.RemoteAddr),
			slog.Time("timestamp", time.Now()),
			slog.String("request_method", r.Method),
			slog.String("request_url", r.URL.Path),
			slog.String("request_protocol", r.Proto),
			slog.Int("status", status),
			slog.Int("bytes_sent", lrw.bytesWritten),
			slog.String("referer", r.Referer()),
			slog.String("user_agent", r.UserAgent()),
			slog.Duration("duration", duration),
		)
	})
}
