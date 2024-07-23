package gear_test

import (
	"net/http"

	"github.com/mkch/gear"
)

func ExampleHandler() {
	var handler http.Handler = gear.Handler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var g = gear.G(r)
			// Use g here.
			_ = g
		}))
	http.Handle("/", handler)
}

func ExampleHandlerFunc() {
	var handler http.Handler = gear.HandlerFunc(
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
