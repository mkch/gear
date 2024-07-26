package gear

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path"
	"runtime"
	"strings"
	"time"

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

// Stop stops further middleware processing.
// Current middleware is unaffected.
func (g *Gear) Stop() {
	g.stopped = true
}

// Logger used by Gear.
// Do not set a nil Logger, using log level to control output.
var Logger *slog.Logger = slog.Default()

// NoLog returns a Logger discards all messages and has a level of -99.
// The following code disables message logging to a certain extent:
//
//	Logger = NoLog()
func NoLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.Level(-99)}))
}

// logImpl is the helper function to log messag with Logger.
// It must always be called directly by an exported logging method
// or function, because it uses a fixed call depth to obtain the pc.
func logImpl(level slog.Level, msg string, args ...any) {
	if !Logger.Enabled(context.Background(), level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip [wrapper, Callers, logImpl]
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	Logger.Handler().Handle(context.Background(), r)
}

// Log logs at level with [Logger].
func Log(level slog.Level, msg string, args ...any) {
	logImpl(level, msg, args)
}

// LogD logs at [slog.LevelDebug] with [Logger].
func LogD(msg string, args ...any) {
	logImpl(slog.LevelDebug, msg, args...)
}

// LogI logs at [slog.LevelInfo] with [Logger].
func LogI(msg string, args ...any) {
	logImpl(slog.LevelInfo, msg, args...)
}

// LogW logs at [slog.LevelWarn] with [Logger].
func LogW(msg string, args ...any) {
	logImpl(slog.LevelWarn, msg, args...)
}

// LogE logs at [slog.LevelError] with [Logger].
func LogE(msg string, args ...any) {
	logImpl(slog.LevelError, msg, args...)
}

// LogIfErr logs err at [slog.LevelError] with [Logger] if err != nil.
// The log message has attribute {"err":err}.
// This function is convenient to log non-nil return value.
// For example:
//
//	LogIfErr(g.JSON(v))
func LogIfErr(err error) {
	if err != nil {
		logImpl(slog.LevelError, "", "err", err)
	}
}

// LogIfErrT logs ret and err at [slog.LevelError] with [Logger] if err != nil.
// The log message has attribute {"ret": ret, "err":err}.
// This function is convenient to log non-nil return value.
// For example:
//
//	LogIfErrT(fmt.Println("msg"))
func LogIfErrT[T any](ret T, err error) {
	if err != nil {
		logImpl(slog.LevelError, "", "ret", ret, "err", err)
	}
}

// DecodeBody parses body and stores the result in the value pointed to by v.
// This method is a shortcut of encoding.DecodeBody(g.R, nil, v).
// See [encoding.DecodeBody] for more details.
func (g *Gear) DecodeBody(v any) error {
	return encoding.DecodeBody(g.R, nil, v)
}

// MustDecodeBody calls [Gear.DecodeBody]. If DecodeBody returns an error, MustDecodeBody returns it but also
// writes a http.StatusBadRequest response and stops the middleware processing.
func (g *Gear) MustDecodeBody(v any) (err error) {
	if err = g.DecodeBody(v); err != nil {
		g.Error(http.StatusBadRequest)
		g.Stop()
	}
	return
}

// DecodeFrom calls g.R.ParseForm(), decodes g.R.Form and stores the result in the value pointed by v.
// See [encoding.DecodeForm] for more details.
// Call ParseMultipartForm() of the request to include values in multi-part form.
func (g *Gear) DecodeForm(v any) error {
	LogIfErr(g.R.ParseForm())
	return encoding.DecodeForm(g.R, nil, v)
}

// MustDecodeForm calls [Gear.DecodeForm]. If DecodeForm returns an error, MustDecodeForm returns it but also
// writes a http.StatusBadRequest response and stops the middleware processing.
func (g *Gear) MustDecodeForm(v any) (err error) {
	if err = g.DecodeForm(v); err != nil {
		g.Error(http.StatusBadRequest)
		g.Stop()
	}
	return
}

// WriteError writes error code and status text using http.Error().
func (g *Gear) Error(code int) {
	http.Error(g.W, http.StatusText(code), code)
}

// Write copies data from r to g.W.
func (g *Gear) Write(r io.Reader) error {
	_, err := io.Copy(g.W, r)
	return err
}

// JSON writes JSON encoding ov v to g.W.
func (g *Gear) JSON(v any) error {
	return encoding.EncodeJSON(v, g.W)
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

// Server calls httptest.NewServer with Wrap(handler, middlewares)).
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

// Group is prefix of a group of urls registered to http.ServeMux.
type Group struct {
	mux         *http.ServeMux
	prefix      string
	middlewares []Middleware
}

// NewGroup create a prefix of URLs on mux. When any URL has the prefix is requested,
// middlewares of group handle the request before URL handler.
// If mux is nil, http.DefaultServeMux will be used.
func NewGroup(prefix string, mux *http.ServeMux, middlewares ...Middleware) *Group {
	if mux == nil {
		mux = http.DefaultServeMux
	}
	return &Group{mux, prefix, middlewares}
}

// emptyHttpHandler is a http.Handler does nothing.
var emptyHttpHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { /*nop*/ })

// Handle registers handler for a pattern which is the group prefix joined ([path.Join]) pattern parameter.
// The handler and middlewares are wrapped(see [Wrap]) before registering.
// Group's middlewares take precedence over the wrapped handler here.
// If handler is nil, an empty handler will be used.
func (group *Group) Handle(pattern string, handler http.Handler, middlewares ...Middleware) *Group {
	if handler == nil {
		handler = emptyHttpHandler
	}
	group.mux.Handle(path.Join(group.prefix, pattern),
		Wrap(handler,
			append(middlewares, group.middlewares...)...)) // group middlewares take precedence.
	return group
}

// Group creates a new URL prefix: path.Join(parent.prefix, prefix).
// When any URL has the prefix is requested, middlewares of parent group
// handle the request before the new group.
func (parent *Group) Group(prefix string, middlewares ...Middleware) *Group {
	return &Group{
		parent.mux,
		path.Join(parent.prefix, prefix),
		append(middlewares, parent.middlewares...), // parent group takes precedence.
	}
}
