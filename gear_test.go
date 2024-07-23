package gear_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/mkch/gear"
	"github.com/mkch/gear/impl/geartest"
)

func TestJSONBodyDecoder(t *testing.T) {
	var respBody = "abc\ndef"
	var mux http.ServeMux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
	server := gear.NewTestServer(&mux)
	defer server.Close()

	body, vars := geartest.CurlPOST(server.URL, "application/json", `{"N":1, "S":"str"}`, "-w", "\n%{http_code}")
	if code := vars["response_code"]; code.(float64) != float64(200) {
		t.Fatal(code)
	}
	if string(body) != respBody {
		t.Fatal(string(body))
	}
}

func TestMiddleWare(t *testing.T) {
	var logs []string
	var logMiddleware = gear.MiddlewareFunc(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logs = append(logs, fmt.Sprintf("Before request: Path=%v", r.URL.Path))
			h.ServeHTTP(w, r)
			logs = append(logs, fmt.Sprintf("After request: Path=%v", r.URL.Path))
		})
	})

	var mux http.ServeMux

	server := gear.NewTestServer(&mux, logMiddleware)
	defer server.Close()

	geartest.Curl(server.URL)

	if len(logs) != 2 {
		t.Fatal(logs)
	} else if pre := logs[0]; pre != "Before request: Path=/" {
		t.Fatal(pre)
	} else if post := logs[1]; post != "After request: Path=/" {
		t.Fatal(post)
	}
}
