package encoding

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"strconv"

	"github.com/mkch/gg"
)

// FormDecoder decodes form values, such as [http.Request.Form], [http.Request.PostForm].
//
// DecodeForm method works like [json.Unmarshal].
// It parses [url.Values] and stores the result in the value pointed by v.
// if v is nil or not a pointer, DecodeForm returns an [InvalidDecodeError].
//
// The parameter v can be one of the following types.
//   - *map[string][]string : *v is a copy of values.
//   - *map[string]string   : *v has the same content of values but each pair only has the firs value.
//   - *map[string]any      : *v has the same content as above but with any value type.
//
// or any *struct type. The struct field can be one of the following types.
//   - string
//   - integers(int8, int18, uint, uintptr etc).
//   - floats(float32, float64).
//   - Pointers or slices of the the above.
//   - Type implements [FormValueUnmarshaler].
//
// A Value is converted to the type of the field, if conversion failed, an [DecodeFieldError] will be returned.
// Slices and pointers are allocated as necessary. A Slice field contains all the values of the key,
// non-slice field contains the first value only. A FormValueUnmarshaler decodes []string into itself.
//
// The follow field tags can be used:
//   - `form:"key_name"` : key_name is the name of the key.
//   - `form:"-"`        : this field is ignored.
type FormDecoder interface {
	DecodeForm(values url.Values, v any) error
}

// FormValueUnmarshaler is the interface implemented by types that can unmarshal form []string.
// The input is the value of url.Values. UnmarshalFormValue must copy the slice if it wishes
// to retain the data after returning.
type FormValueUnmarshaler interface {
	UnmarshalFormValue([]string) error
}

// FormDecoderFunc is an adapter to allow the use of ordinary functions as [FormDecoder].
// If f is a function with the appropriate signature, FormDecoderFunc(f) is a FormDecoder that calls f.
type FormDecoderFunc func(values url.Values, v any) error

func (f FormDecoderFunc) DecodeForm(values url.Values, v any) error {
	return f(values, v)
}

// An InvalidDecodeError describes an invalid argument passed to FormDecoder.DecodeForm().
// The argument to decode must be a non-nil pointer.
type InvalidDecodeError struct {
	Type reflect.Type
}

func (e *InvalidDecodeError) Error() string {
	if e.Type == nil {
		return "gear: Decode(nil)"
	}

	if e.Type.Kind() != reflect.Pointer {
		return "gear: Decode(non-pointer " + e.Type.String() + ")"
	}
	return "gear: Decode(nil " + e.Type.String() + ")"
}

// An DecodeTypeError is returned by FormDecoder.DecodeForm, describing a type that can't be decoded into.
type DecodeTypeError struct {
	Type reflect.Type
}

func (err *DecodeTypeError) Error() string {
	return "gear: cannot decode into " + err.Type.String()
}

// An DecodeAddressError is returned by FormDecoder.DecodeForm, describing a value that is not addressable.
type DecodeAddressError struct {
	Type reflect.Type
}

func (err *DecodeAddressError) Error() string {
	return "gear: cannot decode into unaddressable value " + err.Type.String() + " value"
}

// An DecodeFieldError is returned by FormDecoder.DecodeForm, describing a value that can't convert to the type of field.
type DecodeFieldError struct {
	Name  string
	Type  reflect.Type
	Value string
	Err   error
}

func (e *DecodeFieldError) Error() string {
	ret := "gear: cannot decode " + fmt.Sprintf("%#v", e.Value) + " as " + e.Type.String() + " into field " + e.Name
	if e.Err != nil {
		ret += ": " + e.Err.Error()
	}
	return ret
}

// DecodeForm decodes r.Form using decoder and stores the result in the value pointed by v.
// If decoder is nil, [DefaultFormDecoder] will be used.
// Note: r.ParseForm or ParseMultipartForm should be call to populate r.Form.
func DecodeForm(r *http.Request, decoder FormDecoder, v any) (err error) {
	if decoder == nil {
		decoder = DefaultFormDecoder
	}
	return decoder.DecodeForm(r.Form, v)
}

// DefaultFormDecoder is the default implementation of [FormDecoder].
var DefaultFormDecoder = FormDecoderFunc(decodeForm)

// decodeForm is the default implementation of [FormDecoder.DecodeForm].
func decodeForm(values url.Values, v any) error {
	typ := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if typ == nil || typ.Kind() != reflect.Pointer || !val.IsValid() {
		return &InvalidDecodeError{typ}
	}
	// Indirections.
	typ = typ.Elem()
	val = val.Elem()

	if !val.CanSet() {
		return &DecodeAddressError{typ}
	}

	// Special case: simple conversions.
	if p, ok := v.(*map[string][]string); ok {
		if *p == nil {
			*p = make(map[string][]string)
		}
		maps.Copy(*p, values)
		return nil
	}
	if p, ok := v.(*map[string]string); ok {
		if *p == nil {
			*p = make(map[string]string)
		}
		for k := range values {
			(*p)[k] = values.Get(k)
		}
		return nil
	}
	if p, ok := v.(*map[string]any); ok {
		if *p == nil {
			*p = make(map[string]any)
		}
		for k := range values {
			(*p)[k] = values.Get(k)
		}
		return nil
	}

	// Cannot decode into types other than struct.
	if typ.Kind() != reflect.Struct {
		return &DecodeTypeError{typ}
	}

	// Processing struct fields.
	for i, nField := 0, typ.NumField(); i < nField; i++ {
		field := typ.Field(i)
		if !field.IsExported() || field.Anonymous {
			continue
		}
		tag := field.Tag.Get("form")
		if tag == "-" {
			continue // ignore
		}
		// key to map
		var key string = gg.If(tag != "", tag, field.Name)
		if !values.Has(key) {
			continue // key not found
		}
		if err := parseFormValue(values[key], val.Field(i)); err != nil {
			err.Name = field.Name
			return err
		}
	}
	return nil
}

var formUnmarshalerType = reflect.TypeOf((*FormValueUnmarshaler)(nil)).Elem()

// parseFormValue parses values into dest. Return non-nil if error occurs.
// If err is not nil, the Name field is not set(unknown in this function).
func parseFormValue(values []string, dest reflect.Value) *DecodeFieldError {
	var err error
	t := dest.Type()
	if t.Implements(formUnmarshalerType) {
		if t.Kind() == reflect.Pointer && dest.IsNil() {
			dest.Set(reflect.New(t.Elem()))
		}
		unmarshaler := dest.Interface().(FormValueUnmarshaler)
		err = unmarshaler.UnmarshalFormValue(values)
		if err != nil {
			return &DecodeFieldError{Type: t, Value: fmt.Sprintf("%v", values), Err: err}
		}
		return nil
	}

	var value string // The first value in values.
	if len(values) > 0 {
		value = values[0]
	}
	switch t.Kind() {
	case reflect.Pointer:
		var p = reflect.New(t.Elem())                            // alloc
		if err := parseFormValue(values, p.Elem()); err != nil { // parse recursively
			return err
		} else {
			dest.Set(p)
		}
	case reflect.Slice:
		s := dest
		for i := range values {
			var p = reflect.New(t.Elem())                                   // alloc
			if err := parseFormValue(values[i:i+1], p.Elem()); err != nil { // parse recursively
				return err
			} else {
				s = reflect.Append(s, p.Elem())
			}
		}
		dest.Set(s)
	case reflect.Bool:
		dest.SetBool(parseFormBool(value))
		return nil
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		var n int64
		if n, err = strconv.ParseInt(value, 0, int(t.Size()*8)); err == nil {
			dest.SetInt(n)
		}
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.Uintptr:
		var n uint64
		if n, err = strconv.ParseUint(value, 0, int(t.Size()*8)); err == nil {
			dest.SetUint(n)
		}
	case reflect.String:
		dest.SetString(value)
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		var f float64
		if f, err = strconv.ParseFloat(value, int(t.Size()*8)); err == nil {
			dest.SetFloat(f)
		}
	default:
		return &DecodeFieldError{Type: t, Value: value}
	}
	if err != nil {
		return &DecodeFieldError{Type: t, Value: value, Err: err}
	}
	return nil
}

// parseBool parse a form value to bool.
// If it can be parsed using strconv.ParseBool() without error,
// the parsed value is returned. Otherwise true is returned: presence means true.
func parseFormBool(str string) bool {
	b, err := strconv.ParseBool(str)
	if err == nil {
		return b
	}
	return true // presence means true
}
