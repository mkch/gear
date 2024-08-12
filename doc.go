/*
Package gear is the implementation of Gear framewrok.

Gear is about two things: middleware and encoding(binding).

1. Middleware

A middleware is a function that wraps another HTTP handler and performs
some action before or after the wrapped handler executes.

A middleware in Gear is the [Middleware] interface, the wrapping is implemented by
the [Wrap] and other Wrap... functions.

For example the following code add logging to a handler:

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle the request
	})

	handler = gear.Wrap(handler, gear.Logger(nil))

or if you want to log the entire server:

	var server = &http.Server{}
	gear.WrapServer(server)

or listen and serve with logger:

	gear.ListenAndServe("", nil, gear.Logger(nil))

Here is another example of doing authentication:

	var authMiddleware = gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		if !adminAuth(g.R) {
			// Authentication failed, sends 401 and skips the handler.
			g.Code(http.StatusUnauthorized)
			return
		}
		// Executes the handler.
		next(g)
	})

	gear.NewGroup("/admin", nil, authMiddleware).
		HandleFunc("action1", func(w http.ResponseWriter, r *http.Request) {
			// Do action1 of administrator
		}).
		HandleFunc("action2", func(w http.ResponseWriter, r *http.Request) {
			// Do action2 of administrator
		})
	gear.ListenAndServe("", nil)

See package examples section for more examples.
*/
package gear
