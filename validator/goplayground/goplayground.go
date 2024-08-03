/*
Package goplayground implements github.com/go-playground/validator/v10 Validator.
This package registers validator in initializing. So it suffice to have

	import _ "github.com/mkch/gear/validator/goplayground"
*/
package goplayground

import (
	"errors"

	impl "github.com/go-playground/validator/v10"
	"github.com/mkch/gear/validator"
)

var A int

var v = impl.New(impl.WithRequiredStructEnabled())

// validatorFunc is an adapter to allow the use of ordinary functions as [validator.Validator].
type validatorFunc func(any) error

func (f validatorFunc) Struct(s any) error {
	return f(s)
}

func (f validatorFunc) String() string {
	return "github.com/go-playground/validator/v10"
}

func validateStruct(s any) error {
	err := v.Struct(s)
	if err == nil {
		return nil
	}
	var gpvInvalid *impl.InvalidValidationError
	if errors.As(err, &gpvInvalid) {
		return &validator.InvalidValidationError{Type: gpvInvalid.Type}
	}
	return err
}

func init() {
	validator.Register(validatorFunc(validateStruct))
}
