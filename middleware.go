package gear

import (
	"io"
	"net/http"
)

// Middleware is a middleware used in Gear framework.
type Middleware interface {
	// Serve serves http and optionally pass control to next middleware by calling next(g).
	Serve(g *Gear, next func(*Gear))
}

// MiddlewareName is an optional interface to be implemented by a [Middleware].
// If a [Middleware] implements this interface, MiddlewareName() will be used
// to get it's name, or the reflect type name will be used.
type MiddlewareName interface {
	MiddlewareName() string
}

// panicRecovery is the default [Middleware] recovers from panics.
// It sends 500 response.
type panicRecovery struct{}

// Serve implements [Middleware].
func (p panicRecovery) Serve(g *Gear, next func(*Gear)) {
	defer func() {
		v := recover()
		if v != nil {
			RawLogger.Error("recovered from panic", "value", v)
			g.Error(http.StatusInternalServerError)
			g.Stop()
		}
	}()
	next(g)
}

// MiddlewareName implements [MiddlewareName].
func (p panicRecovery) MiddlewareName() string {
	return "PanicRecover"
}

// PanicRecovery returns a [Middleware] which recovers from panics, sends 500 response and print
// "recovered from panic: panic_value" to Logger, send 500 responses.
// Panic recovery middleware should be added as the last middleware to catch all panics.
func PanicRecovery(w io.Writer) Middleware {
	return panicRecovery{}
}

// func middlewareName(m Middleware) string {
// 	if n, ok := m.(MiddlewareName); ok {
// 		return n.MiddlewareName()
// 	}
// 	return reflect.TypeOf(m).String()
// }

// middlewareFunc wraps f and it's middleware name.
// Used by MiddlewareFunc() function.
type middlewareFunc struct {
	name string
	f    func(g *Gear, next func(*Gear))
}

// Serve implements Serve() method of [Middleware].
func (m middlewareFunc) Serve(g *Gear, next func(*Gear)) {
	m.f(g, next)
}

// MiddlewareName implements MiddlewareName() method of [MiddlewareName].
func (m middlewareFunc) MiddlewareName() string {
	return m.name
}

// MiddlewareFuncWitName is an adapter to allow the use of ordinary functions as [Middleware].
// Parameter name will be used as the name of Middleware.
func MiddlewareFuncWitName(f func(g *Gear, next func(*Gear)), name string) Middleware {
	return middlewareFunc{name, f}
}

// MiddlewareFunc is an adapter to allow the use of ordinary functions as [Middleware].
// If f is a function with the appropriate signature, MiddlewareFunc(f) is a Middleware that calls f.
type MiddlewareFunc func(g *Gear, next func(*Gear))

// Serve implements Serve() method of [Middleware].
func (m MiddlewareFunc) Serve(g *Gear, next func(*Gear)) {
	m(g, next)
}

// mwExec is a executable bunch of Middlewares.
type mwExec struct {
	handler     http.Handler // Wrapped handler.
	middlewares []Middleware // Middlewares to handle the request.
	i           int          // Index of current middleware.
}

// newMwExec create a mwExec whose exec() method call a chain of middlewares in reverse order
// and the first middleware calls handler.
func newMwExec(middlewares []Middleware, handler http.Handler) *mwExec {
	var m = &mwExec{handler: handler}
	m.middlewares = middlewares
	if n := len(m.middlewares); n > 0 {
		m.i = n - 1 // Serve from the last to the first.
	}
	return m
}

// exec executes m.
func (m *mwExec) exec(g *Gear) {
	if g.stopped {
		return
	}
	if len(m.middlewares) == 0 {
		m.handler.ServeHTTP(g.W, g.R)
	} else {
		m.serveMiddlewares(g)
	}
}

// serveMiddlewares executes m.middlewares.
func (m *mwExec) serveMiddlewares(g *Gear) {
	m.middlewares[m.i].Serve(g, func(g *Gear) {
		if g.stopped {
			return
		}
		m.i--
		if m.i >= 0 { // Has next.
			m.serveMiddlewares(g) // Call next.
		} else {
			m.handler.ServeHTTP(g.W, g.R) // Call wrapped handler.
		}
	})
}
