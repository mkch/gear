package encoding_test

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/mkch/gear/encoding"
)

func ExampleBodyDecoder() {
	var r *http.Request // From somewhere else.
	// JSONBodyDecoder decodes body into JSON object.
	var JSONBodyDecoder = encoding.BodyDecoderFunc(func(body io.Reader, v any) error {
		return json.NewDecoder(body).Decode(v)
	})

	var object struct {
		Code int
		Msg  string
	}
	encoding.DecodeBody(r, JSONBodyDecoder, &object)
}
