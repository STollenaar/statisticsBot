package routes

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/disgoorg/disgo/bot"
)

var (
	client *bot.Client
)

func CreateRouter(c *bot.Client) {
	client = c

	mux := http.NewServeMux()

	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"message": "pong"})
	})

	addGetUserMessages(mux)
	addFixMessages(mux)
	addFixEmojis(mux)

	log.Println("starting server on :8080")
	_ = http.ListenAndServe(":8080", withMiddleware(mux))
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
				log.Printf("panic %s %s: %v", r.Method, r.URL.Path, rec)
			}
			log.Printf("%d %s %s (%s)", lw.status, r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
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