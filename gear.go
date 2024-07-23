package gear

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/mkch/gear/impl"
)

type contextKey string

// ctxKey is the context key of **Gear in http.Request.Context().
const ctxKey contextKey = "gear"

// Gear, the core of this framework.
type Gear struct {
	R *http.Request
	W http.ResponseWriter
}

// DecodeBody parses body and stores the result in the value pointed to by v.
// This method is a shortcut of
//
// impl.DecodeBody(g.R, nil, v)
//
// See [impl.DecodeBody] for more details.
func (g *Gear) DecodeBody(v any) error {
	return impl.DecodeBody(g.R, nil, v)
}

func newGear(r *http.Request, w http.ResponseWriter) *Gear {
	return &Gear{r, w}
}

// G retrives the Gear in r. It panics if no Gear in request.
func G(r *http.Request) *Gear {
	if g := getGear(r); g == nil {
		panic(errors.New("no gear in request, see gear.Wrap()"))
	} else {
		return *g.(**Gear)
	}
}

func getGear(r *http.Request) any {
	return r.Context().Value(ctxKey)
}

// Handler wraps handler and add Gear to it.
// If handler is nil, http.DefaultServeMux will be used.
func Handler(handler http.Handler) http.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if getGear(r) == nil {
			ctx := r.Context()
			var p *Gear
			ctx = context.WithValue(ctx, ctxKey, &p)
			r = r.WithContext(ctx)
			p = newGear(r, w)
		}
		handler.ServeHTTP(w, r)
	})
}

// HandlerFunc wraps f to a handler and add Gear to it.
// If f is nil, http.DefaultServeMux.ServeHTTP will be used.
func HandlerFunc(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
	if f == nil {
		f = http.DefaultServeMux.ServeHTTP
	}
	return Handler(http.HandlerFunc(f))
}

// ListenAndServe calls http.ListenAndServe(addr, Handler(handler)).
func ListenAndServe(addr string, handler http.Handler) error {
	return http.ListenAndServe(addr, Handler(handler))
}

// ListenAndServe calls http.ListenAndServeTLS(addr, certFile, keyFile, Handler(handler)).
func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler) error {
	return http.ListenAndServeTLS(addr, certFile, keyFile, Handler(handler))
}

// Server wraps server.Handler using Handler() and returns server itself.
func Server(server *http.Server) *http.Server {
	server.Handler = Handler(server.Handler)
	return server
}

// Server calls httptest.NewServer(Handler(handler)).
func NewTestServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(Handler(handler))
}
