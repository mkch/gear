package impl_test

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/mkch/gear/impl"
)

func ExampleBodyDecoder() {
	var r *http.Request // From somewhere else.
	// JSONBodyDecoder decodes body into JSON object.
	var JSONBodyDecoder = impl.BodyDecoderFunc(func(body io.Reader, v any) error {
		return json.NewDecoder(body).Decode(v)
	})

	var object struct {
		Code int
		Msg  string
	}
	impl.DecodeBody(r, JSONBodyDecoder, &object)
}
