package gear

import (
	"context"
	"errors"
	"fmt"
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

// SetContextValue sets the request context value associated with key to val.
func (g *Gear) SetContextValue(key, val any) {
	g.R = g.R.WithContext(context.WithValue(g.R.Context(), key, val))
}

// ContextValue returns the request context value associated with key,
// or nil if no value is associated with key.
func (g *Gear) ContextValue(key any) any {
	return g.R.Context().Value(key)
}

// Stop stops further middleware processing.
// Current middleware is unaffected.
func (g *Gear) Stop() {
	g.stopped = true
}

// RawLogger used by Gear.
// Do not set a nil Logger, using log level to control output.
// See [NoLog].
var RawLogger *slog.Logger = slog.Default()

// NoLog returns a Logger discards all messages and has a level of -99.
// The following code disables message logging to a certain extent:
//
//	RawLogger = NoLog()
func NoLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.Level(-99)}))
}

// logImpl is the helper function to log messag with Logger.
// It must always be called directly by an exported logging method
// or function, because it uses a fixed call depth to obtain the pc.
func logImpl(level slog.Level, msg string, args ...any) {
	if !RawLogger.Enabled(context.Background(), level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // skip [wrapper, Callers, logImpl]
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	RawLogger.Handler().Handle(context.Background(), r)
}

// Log logs at level with [RawLogger].
func Log(level slog.Level, msg string, args ...any) {
	logImpl(level, msg, args)
}

// LogD logs at [slog.LevelDebug] with [RawLogger].
func LogD(msg string, args ...any) {
	logImpl(slog.LevelDebug, msg, args...)
}

// LogI logs at [slog.LevelInfo] with [RawLogger].
func LogI(msg string, args ...any) {
	logImpl(slog.LevelInfo, msg, args...)
}

// LogW logs at [slog.LevelWarn] with [RawLogger].
func LogW(msg string, args ...any) {
	logImpl(slog.LevelWarn, msg, args...)
}

// LogE logs at [slog.LevelError] with [RawLogger].
func LogE(msg string, args ...any) {
	logImpl(slog.LevelError, msg, args...)
}

// LogIfErr logs err at [slog.LevelError] with [RawLogger] if err != nil.
// The log message has attribute {"err":err}. LogIfErr returns err.
// This function is convenient to log non-nil return value.
// For example:
//
//	LogIfErr(g.JSON(v))
func LogIfErr(err error) error {
	if err != nil {
		logImpl(slog.LevelError, "", "err", err)
	}
	return err
}

// LogIfErrT logs ret and err at [slog.LevelError] with [RawLogger] if err != nil.
// The log message has attribute {"ret": ret, "err":err}. LogIfErrorT returns err.
// This function is convenient to log non-nil return value.
// For example:
//
//	LogIfErrT(fmt.Println("msg"))
func LogIfErrT[T any](ret T, err error) error {
	if err != nil {
		logImpl(slog.LevelError, "", "ret", ret, "err", err)
	}
	return err
}

// DecodeBody parses body and stores the result in the value pointed to by v.
// This method is a shortcut of encoding.DecodeBody(g.R, nil, v).
// See [encoding.DecodeBody] for more details.
func (g *Gear) DecodeBody(v any) error {
	return encoding.DecodeBody(g.R, nil, v)
}

// mustDecode calls f(g, v). If f returns an error, mustDecode returns it but also
// writes a http.StatusBadRequest response and stops the middleware processing.
func mustDecode(g *Gear, f func(g *Gear, v any) (err error), v any) (err error) {
	if err = f(g, v); err != nil {
		g.Code(http.StatusBadRequest)
		g.Stop()
	}
	return
}

// MustDecodeBody calls [Gear.DecodeBody]. If DecodeBody returns an error, MustDecodeBody returns it but also
// writes a http.StatusBadRequest response and stops the middleware processing.
func (g *Gear) MustDecodeBody(v any) (err error) {
	return mustDecode(g, (*Gear).DecodeBody, v)
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
	return mustDecode(g, (*Gear).DecodeForm, v)
}

// DecodeHeader decodes g.R.Header and stores the result in the value pointed by v.
// See [encoding.DecodeForm] for more details.
func (g *Gear) DecodeHeader(v any) error {
	return encoding.DecodeHeader(g.R, encoding.HeaderDecoder, v)
}

// MustDecodeHeader calls [Gear.DecodeHeader]. If DecodeHeader returns an error, MustDecodeHeader returns it but also
// writes a http.StatusBadRequest response and stops the middleware processing.
func (g *Gear) MustDecodeHeader(v any) (err error) {
	return mustDecode(g, (*Gear).DecodeHeader, v)
}

// DecodeQuery decodes r.URL.Query() and stores the result in the value pointed by v.
// See [encoding.DecodeForm] for more details.
func (g *Gear) DecodeQuery(v any) error {
	return encoding.DecodeQuery(g.R, encoding.HeaderDecoder, v)
}

// MustDecodeQuery calls [Gear.DecodeQuery]. If DecodeQuery returns an error, MustDecodeHeader returns it but also
// writes a http.StatusBadRequest response and stops the middleware processing.
func (g *Gear) MustDecodeQuery(v any) (err error) {
	return mustDecode(g, (*Gear).DecodeQuery, v)
}

// Code writes code and status text using http.Code().
func (g *Gear) Code(code int) {
	http.Error(g.W, http.StatusText(code), code)
}

// Write copies data from r to the response.
func (g *Gear) Write(r io.Reader) error {
	_, err := io.Copy(g.W, r)
	return err
}

// String writes 200 and body to the response.
func (g *Gear) String(body string) error {
	_, err := io.WriteString(g.W, body)
	return err
}

// StringResponse writes code and body to the response.
func (g *Gear) StringResponse(code int, body string) error {
	g.W.WriteHeader(code)
	_, err := io.WriteString(g.W, body)
	return err
}

// StringResponsef writes code and then call fmt.Fprintf() to write the formated string.
func (g *Gear) StringResponsef(code int, format string, a ...any) error {
	// from http.Error(server.go)
	g.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	g.W.Header().Set("X-Content-Type-Options", "nosniff")
	g.W.WriteHeader(code)
	_, err := fmt.Fprintf(g.W, format+"\n", a...)
	return err
}

// JSON writes JSON encoding of v to the response.
func (g *Gear) JSON(v any) error {
	return encoding.EncodeJSON(v, g.W)
}

// JSONResponse writes code and JSON encoding of v to the response.
func (g *Gear) JSONResponse(code int, v any) error {
	g.W.WriteHeader(code)
	return g.JSON(v)
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

// Wrap wraps handler and adds Gear to it.
// If handler is nil, http.DefaultServeMux will be used.
// Parameter middlewares will be added to the result Handler.
// Middlewares will be served in reversed order of addition,
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

// WrapFunc wraps f to a handler and adds Gear to it.
// If f is nil, http.DefaultServeMux.ServeHTTP will be used.
// Parameter middlewares will be added to the result Handler.
// Middlewares will be served in reversed order of addition ,
// so panic recovery middleware should be added last to catch all panics.
func WrapFunc(f func(w http.ResponseWriter, r *http.Request), middlewares ...Middleware) http.Handler {
	if f == nil {
		f = http.DefaultServeMux.ServeHTTP
	}
	return Wrap(http.HandlerFunc(f), middlewares...)
}

// ListenAndServe calls [http.ListenAndServe](addr, [Wrap](handler, middlewares...)).
// If handler is nil, [http.DefaultServeMux] wil be used.
func ListenAndServe(addr string, handler http.Handler, middlewares ...Middleware) error {
	return http.ListenAndServe(addr, Wrap(handler, middlewares...))
}

// ListenAndServe calls [http.ListenAndServeTLS](addr, certFile, keyFile, [Wrap](handler, middlewares...)).
// If handler is nil, [http.DefaultServeMux] wil be used.
func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler, middlewares ...Middleware) error {
	return http.ListenAndServeTLS(addr, certFile, keyFile, Wrap(handler, middlewares...))
}

// WrapServer wraps server.Handler using [Wrap]() and returns server itself.
func WrapServer(server *http.Server, middlewares ...Middleware) *http.Server {
	server.Handler = Wrap(server.Handler, middlewares...)
	return server
}

// Server calls [httptest.NewServer]() with [Wrap](handler, middlewares...)).
func NewTestServer(handler http.Handler, middlewares ...Middleware) *httptest.Server {
	return httptest.NewServer(Wrap(handler, middlewares...))
}

// PathInterceptor is a [Middleware] intercepting requests with matching URLs.
type PathInterceptor struct {
	prefix      string
	prefixSlash string
	handler     Middleware
}

// NewPathInterceptor returns a [PathInterceptor] which executes handler when the
// path of request URL contains prefix.
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

// HandleFunc converts f to [http.HandlerFunc] and then call [Handle].
func (group *Group) HandleFunc(pattern string, f func(w http.ResponseWriter, r *http.Request), middlewares ...Middleware) *Group {
	return group.Handle(pattern, http.HandlerFunc(f), middlewares...)
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
