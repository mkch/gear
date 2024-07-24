package gear_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
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
	var logMiddleware = gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		logs = append(logs, fmt.Sprintf("Before request: Path=%v", g.R.URL.Path))
		next(g)
		logs = append(logs, fmt.Sprintf("After request: Path=%v", g.R.URL.Path))
	}, "log")

	var mux http.ServeMux

	server := gear.NewTestServer(&mux, logMiddleware)
	defer server.Close()

	geartest.Curl(server.URL)

	if len(logs) != 2 {
		t.Fatal(logs)
	}
	if pre := logs[0]; pre != "Before request: Path=/" {
		t.Fatal(pre)
	}
	if post := logs[1]; post != "After request: Path=/" {
		t.Fatal(post)
	}
}

type Logger []string

func (l *Logger) Serve(g *gear.Gear, next func(*gear.Gear)) {
	*l = append(*l, fmt.Sprintf("Method: %v Path: %v", g.R.Method, g.R.URL.Path))
	next(g)
}

func TestLogger(t *testing.T) {
	var logger Logger
	var mux http.ServeMux
	server := gear.NewTestServer(&mux, &logger)
	defer server.Close()

	geartest.Curl(server.URL)

	if log := logger[0]; log != "Method: GET Path: /" {
		t.Fatal(log)
	}
}

type LoggerWithName struct {
	Logger
}

func (l *LoggerWithName) MiddlewareName() string {
	return "MyLogger"
}

func TestMiddlewareName(t *testing.T) {
	var logger LoggerWithName
	var mux http.ServeMux

	var oldWriter = gear.DefaultLogWriter
	var w = &bytes.Buffer{}
	gear.DefaultLogWriter = w
	defer func() { gear.DefaultLogWriter = oldWriter }()

	var oldDebug = gear.LogDebug
	gear.LogDebug = true
	defer func() { gear.LogDebug = oldDebug }()

	server := gear.NewTestServer(&mux, &logger)
	defer server.Close()

	if output := w.String(); !strings.HasSuffix(output, "Middleware added: MyLogger\n") {
		t.Fatal(output)
	}
}

func TestPanicRecover(t *testing.T) {
	var logger Logger
	var mux http.ServeMux

	var oldWriter = gear.DefaultLogWriter
	var w = &bytes.Buffer{}
	gear.DefaultLogWriter = w
	defer func() { gear.DefaultLogWriter = oldWriter }()

	server := gear.NewTestServer(&mux, gear.PanicRecovery(nil), &logger)
	defer server.Close()

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		panic("some error")
	})

	geartest.Curl(path.Join(server.URL, "/error"))
	geartest.Curl(server.URL)

	if output := w.String(); !strings.HasSuffix(output, "recovered from panic: some error\n") {
		t.Fatal(output)
	}

	if len(logger) != 2 {
		t.Fatal(logger)
	}

	if log := logger[0]; log != "Method: GET Path: /error" {
		t.Fatal(log)
	}
	if log := logger[1]; log != "Method: GET Path: /" {
		t.Fatal(log)
	}

}
