package routes

import (
	"bufio"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/disgoorg/disgo/bot"
)

var (
	client *bot.Client
)

// CreateRouter builds the internal API and starts serving it in the background,
// returning the server so the caller can shut it down gracefully. The listener
// is created here rather than in a goroutine the caller launches, so the
// returned server is always safe to call Shutdown on.
func CreateRouter(c *bot.Client) *http.Server {
	client = c

	mux := http.NewServeMux()

	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"message": "pong"})
	})

	addGetUserMessages(mux)
	addFixMessages(mux)
	addFixEmojis(mux)
	addFixEmbeddings(mux)
	addFixMoods(mux)
	addBackup(mux)

	resolveAuth()

	// Auth runs inside the logging/recovery middleware so rejected requests are
	// still logged.
	srv := &http.Server{
		Addr:    ":8080",
		Handler: withMiddleware(withAuth(mux)),
	}

	go func() {
		slog.Info("starting server on :8080")
		// ErrServerClosed is the expected result of a graceful Shutdown, not a
		// failure worth logging as one.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api server failed", slog.Any("err", err))
		}
	}()

	return srv
}

// withMiddleware adds per-request logging and panic recovery (gin.Default's two
// built-ins). The wrapped ResponseWriter forwards Hijack so the /ws upgrade
// still works.
func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lw := &loggingWriter{ResponseWriter: w, status: 200}
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Any("panic", rec))
			}
			slog.Info("request", slog.Int("status", lw.status), slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Duration("took", time.Since(start).Round(time.Millisecond)))
		}()
		next.ServeHTTP(lw, r)
	})
}

// loggingWriter records the status code and preserves http.Hijacker (required
// for WebSocket upgrades) and http.Flusher.
type loggingWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (w *loggingWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
