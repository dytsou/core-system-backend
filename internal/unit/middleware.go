package unit

import (
	"net/http"
)

func SetSearchPathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := r.Header.Get("X-Tenant")
		_ = tenant
		next.ServeHTTP(w, r)
	})
}
