package gear_test

import (
	"fmt"
	"log"

	"github.com/mkch/gear"
)

// LogMiddleware is a middleware to log HTTP message.
type LogMiddleware log.Logger

// Serve implements gear.Middleware.
func (l *LogMiddleware) Serve(g *gear.Gear, next func(*gear.Gear)) {
	fmt.Printf("%v %v", g.R.Method, g.R.URL)
	// Call the real handler.
	next(g)
}

func ExampleMiddleware() {
	// Use LogMiddleware.
	gear.ListenAndServe(":80", nil, (*LogMiddleware)(log.Default()))
}
