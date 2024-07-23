package gear

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/mkch/gear/impl"
)

type contextKey string

// ctxKey is the context key of *Gear in http.Request.Context().
const ctxKey contextKey = "gear"

// Gear, the core of this framework.
type Gear struct {
	R       *http.Request       // R of this request.
	W       http.ResponseWriter // W of this request.
	handler http.Handler        // The middleware to handle the request.
}

// addMiddleware adds middlewares to g.
func (g *Gear) addMiddleware(middlewares []Middleware) {
	for _, mw := range middlewares {
		g.handler = mw.Wrap(g.handler)
	}
}

// Middleware is a middleware used in Gear framework.
type Middleware interface {
	// Wrap warps h and intercepts its behavior.
	Wrap(h http.Handler) http.Handler
}

// MiddlewareFunc is an adapter to allow the use of ordinary functions as [Middleware].
// If f is a function with the appropriate signature, MiddlewareFunc(f) is a Middleware that calls f.
type MiddlewareFunc func(h http.Handler) http.Handler

// Wrap implements Wrap() method of [Middleware].
func (f MiddlewareFunc) Wrap(h http.Handler) http.Handler {
	return f(h)
}

// DecodeBody parses body and stores the result in the value pointed to by v.
// This method is a shortcut of impl.DecodeBody(g.R, nil, v).
// See [impl.DecodeBody] for more details.
func (g *Gear) DecodeBody(v any) error {
	return impl.DecodeBody(g.R, nil, v)
}

// G retrives the Gear in r. It panics if no Gear in request.
func G(r *http.Request) *Gear {
	if g := getGear(r); g == nil {
		panic(errors.New("no gear in request, see gear.Wrap()"))
	} else {
		return g.(*Gear)
	}
}

func getGear(r *http.Request) any {
	return r.Context().Value(ctxKey)
}

// Wrap wraps handler and add Gear to it.
// If handler is nil, http.DefaultServeMux will be used.
// Parameter middlewares will be added to Gear if not empty.
func Wrap(handler http.Handler, middlewares ...Middleware) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var g *Gear
		if val := getGear(r); val == nil {
			// No gear.
			g = &Gear{handler: handler}
			g.addMiddleware(middlewares)
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey, g)
			r = r.WithContext(ctx)
		} else {
			g = val.(*Gear)
		}
		g.R = r
		g.W = w
		g.handler.ServeHTTP(w, r)
	})
}

// WrapFunc wraps f to a handler and add Gear to it.
// If f is nil, http.DefaultServeMux.ServeHTTP will be used.
// Parameter middlewares will be added to Gear if not empty.
func WrapFunc(f func(w http.ResponseWriter, r *http.Request), middlewares ...Middleware) http.Handler {
	if f == nil {
		f = http.DefaultServeMux.ServeHTTP
	}
	return Wrap(http.HandlerFunc(f), middlewares...)
}

// ListenAndServe calls http.ListenAndServe(addr, Wrap(handler, middlewares...)).
// If handler is nil, http.DefaultServeMux wil be used.
func ListenAndServe(addr string, handler http.Handler, middlewares ...Middleware) error {
	return http.ListenAndServe(addr, Wrap(handler, middlewares...))
}

// ListenAndServe calls http.ListenAndServeTLS(addr, certFile, keyFile, Wrap(handler, middlewares...)).
// If handler is nil, http.DefaultServeMux wil be used.
func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler, middlewares ...Middleware) error {
	return http.ListenAndServeTLS(addr, certFile, keyFile, Wrap(handler, middlewares...))
}

// Server wraps server.Handler using Wrap() and returns server itself.
func Server(server *http.Server, middlewares ...Middleware) *http.Server {
	server.Handler = Wrap(server.Handler, middlewares...)
	return server
}

// Server calls httptest.NewServer(Handler(handler)).
func NewTestServer(handler http.Handler, middlewares ...Middleware) *httptest.Server {
	return httptest.NewServer(Wrap(handler, middlewares...))
}
