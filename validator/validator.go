/*
Package validator provides a generic interface for value validation.
A [Validator] must be [Register] ed before doing validation.

Package github.com/mkch/gear/validator/goplayground provides a [Validator] implementation form github.com/go-playground/validator/v10.

See package [github.com/mkch/gear/validator/goplayground] for an example.
*/
package validator

import (
	"reflect"
)

// Validator is the interface to validate.
type Validator interface {
	// Struct validates a struct value s.
	// If the validation failed, Struct returns an non-nil error describing the reason.
	Struct(s any) error
	// String returns a description of the implementation of this validator.
	// The description will typically contain something like the author,
	// or repository URL and version.
	String() string
}

var validator Validator

// Register sets v as the validator used by this package and
// replaces existing validator if any.
// Register panics if v is nil.
func Register(v Validator) {
	if v == nil {
		panic("nil validator")
	}
	validator = v
}

// Registered returns whether a validator has been registered.
func Registered() bool {
	return validator != nil
}

// MustRegistered panics if no validator has been registered.
func MustRegistered() {
	if !Registered() {
		panic("no validator")
	}
}

// InvalidValidationError records a type that can not be validated.
// Validator implements must return error of this type when the parameter
// can't be validated.
type InvalidValidationError struct {
	Type reflect.Type
}

// String implements error interface.
func (err *InvalidValidationError) Error() string {
	return "validator: invalid type " + err.Type.String()
}

// Struct validates struct s.
// If no validator has been registered, validated is set to false.
// If validated is true, err will be the return value from validator implementation.
func Struct(s any) (validated bool, err error) {
	if validator == nil {
		return false, nil
	}
	return true, validator.Struct(s)
}
