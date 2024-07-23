package gear

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"

	"github.com/mkch/gear/impl"
)

type contextKey string

// ctxKey is the context key of *Gear in http.Request.Context().
const ctxKey contextKey = "gear"

// Gear, the core of this framework.
type Gear struct {
	R            *http.Request       // R of this request.
	W            http.ResponseWriter // W of this request.
	handler      http.Handler        // The middleware to handle the request.
	loggers      []Logger            // All the loggers
	panicRecover PanicRecover
}

// LogInfo passed msg to all LogInfo() methods of logger middlewares.
func (g *Gear) LogInfo(msg string, arg ...any) {
	for _, logger := range g.loggers {
		logger.LogInfo(msg, arg...)
	}
}

// LogError passed msg to all LogError() methods of logger middlewares.
func (g *Gear) LogError(msg string, arg ...any) {
	for _, logger := range g.loggers {
		logger.LogError(msg, arg...)
	}
}

// addMiddleware adds middlewares to g.
func (g *Gear) addMiddleware(middlewares []Middleware) {
	var apply = func(mw Middleware) {
		g.handler = mw.Wrap(g.handler)
		if logger, ok := mw.(Logger); ok {
			g.loggers = append(g.loggers, logger)
		}
		g.LogInfo(fmt.Sprintf("Middleware added: %v", middlewareName(mw)))
	}

	var panicRecover PanicRecover
	for _, mw := range middlewares {
		if r, ok := mw.(PanicRecover); ok {
			if panicRecover != nil {
				panic("only one PanicRecover is supported")
			}
			panicRecover = r
			continue // PanicRecover must be added at the last.
		}
		apply(mw)
	}

	// PanicRecover must be the outermost wrapper.
	if panicRecover != nil {
		apply(panicRecover)
	}
}

// Middleware is a middleware used in Gear framework.
type Middleware interface {
	// Wrap warps h and intercepts its behavior.
	Wrap(h http.Handler) http.Handler
}

// MiddlewareName is an optional interface to be implemented by a [Middleware].
// If a [Middleware] implements this interface, MiddlewareName() will be used
// to get it's name, or the reflect type name will be used.
type MiddlewareName interface {
	MiddlewareName() string
}

// Logger is a [Middleware] that can log general messages.
type Logger interface {
	Middleware
	LogInfo(msg string, args ...any)
	LogError(msg string, args ...any)
}

// PanicRecover is a [Middleware] that recovers from panics.
// A PanicRecover must call the handler set by SetHandler() after recovering.
// At most one PanicRecover can be added to a Handler.
type PanicRecover interface {
	Middleware
	PanicRecover()
}

// panicRecover is the default [PanicRecover] implementation.
type panicRecover struct{}

// Wrap implements [Middleware].
func (p panicRecover) Wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v != nil {
				G(r).LogError("recovered from panic: %v", v)
			}
		}()
		h.ServeHTTP(w, r)
	})
}

// MiddlewareName implements [MiddlewareName].
func (p panicRecover) MiddlewareName() string {
	return "PanicRecover"
}

// DefaultPanicRecover is the default [PanicRecover] implementation which calls
//
// Gear.LogError("recovered from panic: %v", v)
//
// where v is the recovered value.
var DefaultPanicRecover = &panicRecover{}

func middlewareName(m Middleware) string {
	if n, ok := m.(MiddlewareName); ok {
		return n.MiddlewareName()
	}
	return reflect.TypeOf(m).String()
}

// middlewareFunc wraps f and it's middleware name.
// Used by MiddlewareFunc() function.
type middlewareFunc struct {
	name string
	f    func(h http.Handler) http.Handler
}

// Wrap implements Wrap() method of [Middleware].
func (m middlewareFunc) Wrap(h http.Handler) http.Handler {
	return m.f(h)
}

// MiddlewareName implements MiddlewareName() method of [MiddlewareName].
func (m middlewareFunc) MiddlewareName() string {
	return m.name
}

// MiddlewareFunc is an adapter to allow the use of ordinary functions as [Middleware].
// Parameter name will be used as the name of Middleware.
func MiddlewareFunc(f func(http.Handler) http.Handler, name string) Middleware {
	return middlewareFunc{name, f}
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
// Parameter middlewares will be added to the result Handler.
// If there are more than one [PanicRecover] in middlewares, it panics.
func Wrap(handler http.Handler, middlewares ...Middleware) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	var g *Gear = &Gear{handler: handler}
	g.addMiddleware(middlewares)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if val := getGear(r); val != nil {
			panic("already a Gear handler")
		} else {
			// No gear.
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey, g)
			r = r.WithContext(ctx)
		}
		g.R = r
		g.W = w
		g.handler.ServeHTTP(w, r)
	})
}

// WrapFunc wraps f to a handler and add Gear to it.
// If f is nil, http.DefaultServeMux.ServeHTTP will be used.
// Parameter middlewares will be added to the result Handler.
// If there are more than one [PanicRecover] in middlewares, it panics.
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
