package gear

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	pathlib "path"
	"strings"

	"github.com/mkch/gear/impl"
)

type contextKey string

// ctxKey is the context key of *Gear in http.Request.Context().
const ctxKey contextKey = "gear"

// Gear, the core of this framework.
type Gear struct {
	R       *http.Request       // R of this request.
	W       http.ResponseWriter // W of this request.
	stopped bool                // Whether g.Stop() has been called.
}

// Stop stops calling subsequent handling.
// The processing of current middleware is unaffected.
func (g *Gear) Stop() {
	g.stopped = true
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
// Panic recovery middleware should be added as the last middleware to catch all panics.
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
	if len(m.middlewares) == 0 {
		m.handler.ServeHTTP(g.W, g.R)
	} else {
		m.serveMiddlewares(g)
	}
}

// serveMiddlewares executes m.middlewares.
func (m *mwExec) serveMiddlewares(g *Gear) {
	if g.stopped {
		return
	}
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

// Wrap wraps handler and add Gear to it.
// If handler is nil, http.DefaultServeMux will be used.
// Parameter middlewares will be added to the result Handler.
// Middlewares will be called in reversed order of addition,
// so panic recovery middleware should be added last to catch all panics.
func Wrap(handler http.Handler, middlewares ...Middleware) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var g *Gear
		if val := getGear(r); val != nil {
			g = val.(*Gear)
		} else {
			// Add gear.
			g = &Gear{W: w}
			ctx := context.WithValue(r.Context(), ctxKey, g)
			g.R = r.WithContext(ctx)
		}
		newMwExec(middlewares, handler).exec(g)
	})
}

// WrapFunc wraps f to a handler and add Gear to it.
// If f is nil, http.DefaultServeMux.ServeHTTP will be used.
// Parameter middlewares will be added to the result Handler.
// Middlewares will be called in reversed order of addition ,
// so panic recovery middleware should be added last to catch all panics.
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

// WrapServer wraps server.Handler using Wrap() and returns server itself.
func WrapServer(server *http.Server, middlewares ...Middleware) *http.Server {
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

// Group is suffix of a group of urls registered to http.ServeMux.
type Group struct {
	mux         *http.ServeMux
	suffix      string
	middlewares []Middleware
}

// NewGroup create a suffix of urls on mux. When any of these urls is
// requested, middlewares of group handle the request before the url's.
// If mux is nil, http.DefaultServeMux will be used.
func NewGroup(suffix string, mux *http.ServeMux, middlewares ...Middleware) *Group {
	if mux == nil {
		mux = http.DefaultServeMux
	}
	return &Group{mux, suffix, middlewares}
}

// Handle registers middlewares for the given pattern. The group's middlewares handle the
// request before pattern middlewares.
func (group *Group) Handle(pattern string, middlewares ...Middleware) *Group {
	if len(middlewares) == 0 {
		return group
	}
	group.mux.Handle(path.Join(group.suffix, pattern),
		WrapFunc(func(http.ResponseWriter, *http.Request) { /*nop*/ },
			append(middlewares, group.middlewares...)...)) // group middlewares take precedence.
	return group
}

// Group creates a new url suffix: path.Join(group.suffix, suffix).
// When any of these urls is requested, middlewares of the new group
// handle the request before group's before url's.
func (group *Group) Group(suffix string, middlewares ...Middleware) *Group {
	return &Group{
		group.mux,
		path.Join(group.suffix, suffix),
		append(group.middlewares, middlewares...), // new group middlewares take precedence.
	}
}
