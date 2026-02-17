package middleware

import "net/http"

// CORS adds Cross-Origin Resource Sharing headers to allow browser-based clients
// like MCP Inspector to connect to the server.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// When credentials are allowed, the CORS spec requires a specific origin
		// (not wildcard). Echo back the requesting origin, or use wildcard for
		// non-credentialed requests with no Origin header.
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		// Allow common HTTP methods
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// Allow common headers including MCP session header
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id, Authorization")

		// Expose the MCP session header so clients can read it
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		// Allow credentials (cookies, authorization headers)
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Cache preflight requests for 1 hour
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
