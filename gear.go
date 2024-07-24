package gear

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	pathlib "path"
	"slices"
	"strings"

	"github.com/mkch/gear/impl"
)

type contextKey string

// ctxKey is the context key of *Gear in http.Request.Context().
const ctxKey contextKey = "gear"

// Gear, the core of this framework.
type Gear struct {
	R           *http.Request       // R of this request.
	W           http.ResponseWriter // W of this request.
	handler     http.Handler        // The wrapped handler.
	middlewares []Middleware        // The middlewares to handle the request.
	curMW       int                 // The index of current middleware.
	stopped     bool                // Whether g.Stop() has been called.
}

// Stop stops calling subsequent handling.
// The processing of current middleware is unaffected.
func (g *Gear) Stop() {
	g.stopped = true
}

// addMiddleware adds middlewares to g.
func (g *Gear) addMiddleware(middlewares []Middleware) {
	g.middlewares = slices.Clone(middlewares)
	if n := len(g.middlewares); n > 0 {
		g.curMW = n - 1 // Serve from the last to the first.
	}
}

// serve handles a http request.
func (g *Gear) serve() {
	if len(g.middlewares) == 0 {
		g.handler.ServeHTTP(g.W, g.R)
	} else {
		g.serveMiddlewares()
	}
}

// serveMiddlewares executes g.middlewares.
func (g *Gear) serveMiddlewares() {
	if g.stopped {
		return
	}
	g.middlewares[g.curMW].Serve(g, func(g *Gear) {
		if g.stopped {
			return
		}
		g.curMW--
		if g.curMW >= 0 {
			g.serveMiddlewares()
		} else {
			g.handler.ServeHTTP(g.W, g.R)
		}
	})
}

// DefaultLogWriter is the writer where the default log output.
var DefaultLogWriter io.Writer = os.Stderr

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
// It sends 500 response and stops the gear.
type panicRecovery log.Logger

// Serve implements [Middleware].
func (p *panicRecovery) Serve(g *Gear, next func(*Gear)) {
	defer func() {
		v := recover()
		if v != nil {
			(*log.Logger)(p).Printf("recovered from panic: %v", v)
			http.Error(g.W, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			g.Stop()
		}
	}()
	next(g)
}

// MiddlewareName implements [MiddlewareName].
func (p *panicRecovery) MiddlewareName() string {
	return "PanicRecover"
}

// PanicRecovery returns a [Middleware] which recovers from panics, sends 500 response and print
// "recovered from panic: panic_value" to w. If w is nil, [DefaultLogWriter] will be used.
// Panic recover middleware should be added as the last middleware to catch all panics.
func PanicRecovery(w io.Writer) Middleware {
	return (*panicRecovery)(log.New(DefaultLogWriter, "", log.LstdFlags))
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

// DecodeBody parses body and stores the result in the value pointed to by v.
// This method is a shortcut of impl.DecodeBody(g.R, nil, v).
// See [impl.DecodeBody] for more details.
func (g *Gear) DecodeBody(v any) error {
	return impl.DecodeBody(g.R, nil, v)
}

// G retrives the Gear in r. It panics if no Gear.
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
// Middlewares will be called in reversed order of addition,
// so panic recover middleware should be added last to catch all panics.
func Wrap(handler http.Handler, middlewares ...Middleware) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if val := getGear(r); val != nil {
			panic("already a Gear handler")
		} else {
			// No gear.
			var g *Gear = &Gear{
				W:       w,
				handler: handler,
			}
			g.addMiddleware(middlewares)
			ctx := context.WithValue(r.Context(), ctxKey, g)
			g.R = r.WithContext(ctx)
			g.serve()
		}
	})
}

// WrapFunc wraps f to a handler and add Gear to it.
// If f is nil, http.DefaultServeMux.ServeHTTP will be used.
// Parameter middlewares will be added to the result Handler.
// Middlewares will be called in reversed order of addition ,
// so panic recover middleware should be added last to catch all panics.
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

// PathInterceptor is a [Middleware] matches the prefix of request url path.
type PathInterceptor struct {
	prefix      string
	prefixSlash string
	handler     Middleware
}

// NewPathInterceptor returns a [PathInterceptor] that execute handler when the
// request url path contains prefix.
func NewPathInterceptor(prefix string, handler Middleware) *PathInterceptor {
	prefix = pathlib.Clean(prefix)
	pathSlash := prefix
	if !strings.HasSuffix(pathSlash, "/") {
		pathSlash += "/"
	}
	return &PathInterceptor{
		prefix,
		pathSlash,
		handler,
	}
}

// Serve implements Serve() method of [Middleware].
func (m *PathInterceptor) Serve(g *Gear, next func(*Gear)) {
	if g.R.URL.Path == m.prefix || strings.HasPrefix(g.R.URL.Path, m.prefixSlash) {
		m.handler.Serve(g, next)
	}
	next(g)
}
