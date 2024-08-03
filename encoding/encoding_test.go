package encoding_test

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/mkch/gear"
	"github.com/mkch/gear/encoding"
	"github.com/mkch/gear/internal/geartest"
)

func TestDefaultFormDecoder(t *testing.T) {
	var values = url.Values{
		"k1": []string{"1"},
		"K2": []string{"1.2"},
		"K3": []string{"1", "2"},
		"k4": []string{"1", "10.01"},
		"K5": []string{"10", "20"},
		"k6": []string{},
		"k7": []string{"0"},
	}

	type S1 struct {
		K1 int     `map:"k1"`
		K2 float32 `map:"-"`
		K3 []string
		K4 []float64 `map:"k4"`
		K5 *[]*int
		K6 bool `map:"k6"`
		K7 bool `map:"k7"`
		k  int
		K  any
	}

	var s S1
	if err := encoding.FormDecoder.DecodeMap(values, &s); err != nil {
		t.Fatal(err)
	} else {
		var _10 = 10
		var _20 = 20
		if !reflect.DeepEqual(s,
			S1{K1: 1,
				K3: []string{"1", "2"},
				K4: []float64{1, 10.01},
				K5: &[]*int{&_10, &_20},
				K6: true,
				K7: false,
				k:  0,
				K:  nil}) {
			t.Fatal(s)
		}
	}

	var m1 map[string][]string
	if err := encoding.FormDecoder.DecodeMap(values, &m1); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(url.Values(m1), values) {
		t.Fatal(m1)
	}

	var m2 map[string]string
	if err := encoding.FormDecoder.DecodeMap(values, &m2); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(m2, map[string]string{
		"k1": "1",
		"K2": "1.2",
		"K3": "1",
		"k4": "1",
		"K5": "10",
		"k6": "",
		"k7": "0",
	}) {
		t.Fatal(m2)
	}

	var m3 map[string]any
	if err := encoding.FormDecoder.DecodeMap(values, &m3); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(m3, map[string]any{
		"k1": "1",
		"K2": "1.2",
		"K3": "1",
		"k4": "1",
		"K5": "10",
		"k6": "",
		"k7": "0",
	}) {
		t.Fatal(m2)
	}
}

func TestCustomDecoder(t *testing.T) {
	var errCustomDecoder = errors.New("custom")
	// This should take effect and cause gear.G(r).DecodeBody return sentinel error above.
	encoding.JSONBodyDecoder = encoding.BodyDecoderFunc(func(body io.Reader, v any) error {
		return errCustomDecoder
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var v string
		if err := gear.G(r).DecodeBody(&v); err != errCustomDecoder {
			t.Fatal()
		}
	})
	server := gear.NewTestServer(nil)
	defer server.Close()
	geartest.CurlPOST(server.URL, encoding.MIME_JSON, `{}`, "-w", "\n%{http_code}")
}
