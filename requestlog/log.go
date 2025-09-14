package requestlog

import (
	"log/slog"
	"net/http"
	"time"

	"lds.li/web/internal"
	"lds.li/web/requestid"
	"lds.li/web/slogctx"
)

var _ internal.UnwrappableResponseWriter = (*loggingResponseWriter)(nil)

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

func (lrw *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return lrw.ResponseWriter
}

type RequestLogger struct {
	Logger *slog.Logger
}

func (rl *RequestLogger) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a new context with a handle to capture attributes
		ctx, handle := slogctx.WithHandle(r.Context())
		r = r.WithContext(ctx)

		lrw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		status := lrw.status
		if status == 0 {
			status = http.StatusOK
		}

		l := rl.Logger
		if l == nil {
			l = slog.Default()
		}

		attrs := handle.Attrs()

		attrs = append(attrs,
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
		if rid, ok := requestid.FromContext(ctx); ok {
			attrs = append(attrs, slog.String("request_id", rid))
		}

		anyAttrs := make([]any, len(attrs)*2)
		for i, attr := range attrs {
			anyAttrs[i*2] = attr.Key
			anyAttrs[i*2+1] = attr.Value.Any()
		}

		l.InfoContext(r.Context(), "Request Served", anyAttrs...)
	})
}
