package httpapi

import "net/http"

// withCORS wraps a handler with permissive CORS headers so the Next.js dev
// server (http://localhost:3000) can call this API during the hackathon
// demo. No auth/cookies are in play, so a wildcard origin is fine for a POC.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
