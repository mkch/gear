package gear_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/mkch/gear"
	"github.com/mkch/gear/impl/geartest"
)

func TestJSONBodyDecoder(t *testing.T) {
	var respBody = "abc\ndef"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		g := gear.G(r)
		var data struct {
			N int
			S string
		}
		err := g.DecodeBody(&data)
		if err != nil {
			t.Fatal(err)
		}
		if data.N != 1 || data.S != "str" {
			t.Fatal(data)
		}
		io.WriteString(w, respBody)
	})
	server := gear.NewTestServer(http.DefaultServeMux)
	defer server.Close()

	body, vars := geartest.CurlPOST(server.URL, "application/json", `{"N":1, "S":"str"}`, "-w", "\n%{http_code}")
	if code := vars["response_code"]; code.(float64) != float64(200) {
		t.Fatal(code)
	}
	if string(body) != respBody {
		t.Fatal(string(body))
	}
}
