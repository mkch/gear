package gear

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"

	"github.com/mkch/gear/encoding"
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

// DecodeBody parses body and stores the result in the value pointed to by v.
// This method is a shortcut of impl.DecodeBody(g.R, nil, v).
// See [impl.DecodeBody] for more details.
func (g *Gear) DecodeBody(v any) error {
	return encoding.DecodeBody(g.R, nil, v)
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
	prefix = path.Clean(prefix)
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
