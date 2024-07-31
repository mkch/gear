package encoding_test

import (
	"errors"
	"strings"

	"github.com/mkch/gear"
)

type Name struct {
	First string
	Last  string
}

func (n *Name) UnmarshalMapValue(values []string) error {
	if len(values) == 0 {
		return errors.New("empty slice")
	}
	parts := strings.Split(values[0], " ")
	if len(parts) != 2 {
		return errors.New("invalid name format")
	}
	n.First, n.Last = parts[0], parts[1]
	return nil
}

type Person struct {
	Name *Name `form:"name"`
	Age  int16 `form:"age"`
}

func ExampleMapValueUnmarshaler() {
	var g *gear.Gear // From somewhere else.
	var person Person
	g.DecodeForm(&person) // Can decode: /some/path?name=John+Smith&age=20
}
