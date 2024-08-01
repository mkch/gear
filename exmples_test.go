package gear_test

import (
	"fmt"
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

func ExampleWrapServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var g = gear.G(r)
		// Use g here.
		_ = g
	})
	var server = gear.WrapServer(&http.Server{})
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
	var logMiddleware = gear.MiddlewareFuncWitName(func(g *gear.Gear, next func(*gear.Gear)) {
		// Pre-processing.
		log.Printf("Before request: Path=%v", g.R.URL.Path)
		// Call the real handler.
		next(g)
		// Post-processing.
		log.Printf("After request: Path=%v", g.R.URL.Path)
	}, "logger")
	gear.ListenAndServe(":80", nil, logMiddleware)
}

func ExamplePanicRecovery() {
	gear.ListenAndServe(":80", nil, gear.PanicRecovery())
}

func ExamplePathInterceptor() {
	var handler = gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		// Do admin authentication.
		var authOK bool
		if !authOK {
			http.Error(g.W, "", http.StatusUnauthorized)
			g.Stop()
		}
	})
	// "/admin" and all paths starts with "/admin/" will be intercepted by handler.
	gear.ListenAndServe(":80", nil, gear.NewPathInterceptor("/admin", handler))
}

func adminAuth() bool { return false }

func op1() {}

func ExampleGroup() {
	gear.NewGroup("/admin", nil, gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		// Handle admin authentication
		if !adminAuth() {
			http.Error(g.W, "", http.StatusUnauthorized)
			return
		}
		// OK, go ahead.
		next(g)
	})).Handle("op1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The request path will be /admin/op1
		op1() // Do the operation.
	}))
}

func ExampleLogIfErr() {
	var (
		g *gear.Gear
		v any
	) // From somewhere else.
	gear.LogIfErr(g.JSON(v))
}

func ExampleLogIfErrT() {
	gear.LogIfErrT(fmt.Println("msg"))
}

func ExampleLogger() {
	// logger logs HTTP method, host, URL(by default) and User-Agent header for request.
	logger := gear.Logger(&gear.LoggerOptions{
		HeaderKeys: []string{"User-Agent"},
	})
	gear.ListenAndServe(":http", nil, logger)
}
