package gear_test

import (
	"log"
	"net/http"

	"github.com/mkch/gear"
)

// LogMiddleware is a middleware to log HTTP message.
type LogMiddleware log.Logger

// Wrap implements Middleware.Wrap().
func (l *LogMiddleware) Wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log a message.
		(*log.Logger)(l).Printf("Method: %v Path: %v", r.Method, r.URL.Path)
		// Call the original handler.
		h.ServeHTTP(w, r)
	})
}

func ExampleMiddleware() {
	// Use LogMiddleware.
	gear.ListenAndServe("80", nil, (*LogMiddleware)(log.Default()))
}
