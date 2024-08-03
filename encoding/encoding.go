package encoding

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// BodyDecoder docodes body of http request.
type BodyDecoder interface {
	// DecodeBody parses body and stores the result in the value pointed to by v,
	// which must be an arbitrary struct, slice, or string.
	// Well-formed data that does not fit into v is discarded.
	DecodeBody(body io.Reader, v any) error
}

// BodyDecoderFunc is an adapter to allow the use of ordinary functions as [BodyDecoder].
// If f is a function with the appropriate signature, BodyDecoderFunc(f) is a BodyDecoder that calls f.
type BodyDecoderFunc func(body io.Reader, v any) error

func (f BodyDecoderFunc) DecodeBody(body io.Reader, v any) error {
	return f(body, v)
}

// JSONBodyDecoder decodes body as JSON object.
var JSONBodyDecoder BodyDecoder = BodyDecoderFunc(func(body io.Reader, v any) error {
	return json.NewDecoder(body).Decode(v)
})

// XMLBodyDecoder decodes body as XML document.
var XMLBodyDecoder BodyDecoder = BodyDecoderFunc(func(body io.Reader, v any) error {
	return xml.NewDecoder(body).Decode(v)
})

// UnknownContentType is returned by [DecodeBody] if there is no such [BodyDecoder] to decode the body.
type UnknownContentType string

func (err UnknownContentType) Error() string {
	return fmt.Sprintf("known Content-Type %v", string(err))
}

// DecodeBody decodes r.Body using decoder and stores the result in the value pointed to by v.
// If decoder is nil, Content-Type header of r wii be used to select an available decoder.
// If there is no decoder available for that type, [UnknownContentType] error is returned.
// See [BodyDecoder] for details.
func DecodeBody(r *http.Request, decoder BodyDecoder, v any) (err error) {
	if decoder == nil {
		decoder, err = selectBodyDecoder(r)
		if err != nil {
			return
		}
	}
	return decoder.DecodeBody(r.Body, v)
}

const (
	MIME_JSON     = "application/json"
	MIME_XML      = "application/xml"
	MIME_TEXT_XML = "text/xml"
)

// key is the content type.
var bodyDecoders = map[string]BodyDecoder{
	MIME_JSON:     JSONBodyDecoder,
	MIME_XML:      XMLBodyDecoder,
	MIME_TEXT_XML: XMLBodyDecoder,
}

// selectBodyDecoder returns an decoder from bodyDecoders which can decode the
// body of r. The selection is made by Content-Type header.
func selectBodyDecoder(r *http.Request) (decoder BodyDecoder, err error) {
	mime := r.Header.Get("Content-Type")
	if decoder = bodyDecoders[mime]; decoder == nil {
		err = UnknownContentType(mime)
	}
	return
}

// EncodeJSON writes the JSON encoding of v to the stream w.
func EncodeJSON(v any, w io.Writer) error {
	return json.NewEncoder(w).Encode(v)
}
