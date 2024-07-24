package gear_test

import (
	"log"
	"net/http"

	"github.com/mkch/gear"
)

func ExampleWrap() {
	var handler http.Handler = gear.Wrap(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var g = gear.G(r)
			// Use g here.
			_ = g
		}))
	http.Handle("/", handler)
}

func ExampleWrapFunc() {
	var handler http.Handler = gear.WrapFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var g = gear.G(r)
			// Use g here.
			_ = g
		})
	http.Handle("/", handler)
}

func ExampleListenAndServe() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var g = gear.G(r)
		// Use g here.
		_ = g
	})
	gear.ListenAndServe(":8080", nil)
}

func ExampleListenAndServeTLS() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var g = gear.G(r)
		// Use g here.
		_ = g
	})
	gear.ListenAndServeTLS(":8080", "certfile", "keyfile", nil)
}

func ExampleServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var g = gear.G(r)
		// Use g here.
		_ = g
	})
	var server = gear.Server(&http.Server{})
	server.ListenAndServe()
}

func ExampleNewTestServer() {
	var server = gear.NewTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var g = gear.G(r)
		// Use g here.
		_ = g
	}))
	defer server.Close()
}

func ExampleMiddlewareFunc() {
	var logMiddleware = gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		// Pre-processing.
		log.Printf("Before request: Path=%v", g.R.URL.Path)
		// Call the real handler.
		next(g)
		// Post-processing.
		log.Printf("After request: Path=%v", g.R.URL.Path)
	}, "logger")
	gear.ListenAndServe(":80", nil, logMiddleware)
}

func ExamplePanicRecover() {
	gear.ListenAndServe(":80", nil, gear.PanicRecovery(nil))
}
