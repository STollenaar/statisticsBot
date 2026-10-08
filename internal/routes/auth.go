package routes

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/stollenaar/statisticsbot/internal/util"
)

// authGate holds the resolved state of the shared-token check. The token is
// looked up once at startup rather than per request, because resolving it may
// mean a call to AWS Parameter Store.
type authGate struct {
	token string
	// required is true when a token was configured. When false the API serves
	// unauthenticated, which is the behaviour this server has always had.
	required bool
	// broken is true when a token was configured but could not be read. A token
	// was intended, so every request is refused rather than served without one.
	broken bool
}

var gate authGate

// resolveAuth reads the configured token. It never fails startup: the HTTP API
// is an admin surface, and the Discord side of the bot must keep running even
// when it is misconfigured.
func resolveAuth() {
	token, err := util.GetHTTPAuthToken()
	switch {
	case err != nil:
		gate = authGate{broken: true, required: true}
		slog.Error("HTTP auth token is configured but could not be read; the API will refuse every request",
			slog.Any("err", err))
	case token == "":
		gate = authGate{}
		slog.Warn("no HTTP_AUTH_TOKEN set; the internal API is unauthenticated. " +
			"Set HTTP_AUTH_TOKEN or AWS_HTTP_AUTH_TOKEN to require a bearer token")
	default:
		gate = authGate{token: token, required: true}
		slog.Info("internal API requires a bearer token")
	}
}

// withAuth rejects requests that do not carry the shared token.
//
// GET /ping is always allowed: it returns nothing but a literal "pong", and
// leaving it open keeps it usable as a cheap reachability check from inside the
// cluster.
func withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/ping" {
			next.ServeHTTP(w, r)
			return
		}
		if gate.broken {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "server auth is misconfigured",
			})
			return
		}
		if gate.required && !gate.authorized(r) {
			// Deliberately vague: the caller learns only that it needs a valid
			// token, not whether one was sent or how it was wrong.
			w.Header().Set("WWW-Authenticate", `Bearer realm="statisticsbot"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authorized reports whether the request carries the shared token, accepting it
// either as a bearer token or as a bare X-Auth-Token header so a curl in a
// CronJob can stay short.
func (a authGate) authorized(r *http.Request) bool {
	presented := r.Header.Get("X-Auth-Token")
	if presented == "" {
		if after, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); found {
			presented = after
		}
	}
	if presented == "" {
		return false
	}
	// Constant time, so a caller cannot learn the token a byte at a time by
	// measuring how long the comparison takes.
	return subtle.ConstantTimeCompare([]byte(presented), []byte(a.token)) == 1
}
