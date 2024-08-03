package gear

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/mkch/gg"
	runtimegg "github.com/mkch/gg/runtime"
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
type panicRecovery struct {
	// Whether add "stack" attribute.
	addStack bool
}

// Serve implements [Middleware].
func (p panicRecovery) Serve(g *Gear, next func(*Gear)) {
	defer func() {
		v := recover()
		if v != nil {
			var attrs = make([]slog.Attr, 0, gg.If(p.addStack, 2, 1))
			attrs = append(attrs, slog.Any("value", v))
			if p.addStack {
				attrs = append(attrs, slog.Any("stack", runtimegg.Stack(1, 0))) // 1: skip this anonymous function.
			}
			RawLogger.LogAttrs(context.Background(), slog.LevelError, "recovered from panic", attrs...)
			g.Code(http.StatusInternalServerError)
			g.Stop()
		}
	}()
	next(g)
}

// MiddlewareName implements [MiddlewareName].
func (p panicRecovery) MiddlewareName() string {
	return "PanicRecover"
}

// PanicRecovery returns a [Middleware] which recovers from panics,
// logs a LevelError message "recovered from panic" and sends 500 responses.
// The "value" attribute is set to panic value.
// If addStack is true, "stack" attribute is set to the string representation of the call stack.
// Panic recovery middleware should be added as the last middleware to catch all panics.
func PanicRecovery(addStack bool) Middleware {
	return panicRecovery{addStack}
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

const (
	// LoggerMethodKey is the key used by [Logger] for the HTTP method of HTTP request.
	// The associated Value is a string.
	LoggerMethodKey = "method"
	// LoggerMethodKey is the key used by [Logger] for the host of HTTP request.
	// The associated Value is a string.
	LoggerHostKey = "host"
	// LoggerMethodKey is the key used by [Logger] for the URL of HTTP request.
	// The associated Value is a string.
	LoggerURLKey = "URL"
	// LoggerMethodKey is the group key used by [Logger] for the header of HTTP request.
	// The associated Value in group is a string.
	LoggerHeaderKey = "header"
)

// LoggerOptions are options for [Logger]. A zero LoggerOptions consists entirely of zero values.
type LoggerOptions struct {
	// Keys are the keys to log. Keys is a set of strings.
	// Zero value means all Logger keys available(See LoggerMethodKey etc).
	Keys map[string]bool
	// HeaderKeys are the keys of HTTP header to log.
	// HeaderKeys are only used when LoggerHeaderKey is in Keys.
	// Zero value means not logging any header value.
	HeaderKeys []string
	// Attrs can be used to generate the slog.Attr slice to log for r.
	// If Attrs is not nil, all fields above are ignored, the Logger just
	// calls LogAttrs() to log the return value of this function.
	// This function should not retain or modify r.
	Attrs func(r *http.Request) []slog.Attr
}

// Logger returns a [Middleware] to log HTTP access log.
// If opt is nil, the default options are used.
//
// Log level: LevelInfo
//
// Log attributes:
//
//	"msg": "HTTP"
//	"method": request.Method
//	"host": request.Host
//	"URL": request.URL
//	"header.headerKey": request.Header[headerKey]
func Logger(opt *LoggerOptions) Middleware {
	return MiddlewareFuncWitName(func(g *Gear, next func(*Gear)) {
		var attrs []slog.Attr
		if opt != nil && opt.Attrs != nil { // opt.Attrs takes precedency.
			attrs = opt.Attrs(g.R)
		} else {
			// Default values.
			var headerKeys []string
			var logMethod = true
			var logHost = true
			var logURL = true
			// Values in options.
			if opt != nil {
				var logHeader = true
				if opt.Keys != nil {
					logMethod = opt.Keys[LoggerMethodKey]
					logHost = opt.Keys[LoggerHostKey]
					logURL = opt.Keys[LoggerURLKey]
					logHeader = opt.Keys[LoggerHeaderKey]
				}
				if logHeader && opt.HeaderKeys != nil {
					headerKeys = opt.HeaderKeys
				}
			}
			attrs = make([]slog.Attr, 0, 3+gg.If(len(headerKeys) > 0, 1, 0)) // 3: method, host, URL
			if logMethod {
				attrs = append(attrs, slog.String(LoggerMethodKey, g.R.Method))
			}
			if logHost {
				attrs = append(attrs, slog.String(LoggerHostKey, g.R.Host))
			}
			if logURL {
				attrs = append(attrs, slog.Any(LoggerURLKey, g.R.URL))
			}
			if len(headerKeys) > 0 {
				var headers []any = make([]any, 0, len(headerKeys))
				for _, key := range headerKeys {
					headers = append(headers, slog.Any(key, g.R.Header[key]))
				}
				attrs = append(attrs, slog.Group(LoggerHeaderKey, headers...))
			}
		}
		RawLogger.LogAttrs(context.Background(), slog.LevelInfo, "HTTP", attrs...)
		next(g)
	}, "Logger")
}
