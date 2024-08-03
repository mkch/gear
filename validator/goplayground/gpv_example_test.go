package goplayground_test

import (
	"net/http"

	"github.com/mkch/gear"
	_ "github.com/mkch/gear/validator/goplayground" // for side effect
)

type User struct {
	// `validate:"required"` tag marks ID required.
	// See https://pkg.go.dev/github.com/go-playground/validator/v10
	ID   uint `validate:"required"`
	Name string
}

func Example() {
	http.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {

		var user User
		err := gear.G(r).DecodeBody(&user)
		// If the request body does not contains an ID field,
		// err will be the error returned form validator.
		_ = err
	})
}
