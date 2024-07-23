package gear_test

import (
	"fmt"
	"io"
	"net/http"
	"path"
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

func (l *Logger) LogInfo(msg string, arg ...any) {
	(*l) = append(*l, fmt.Sprintf(msg, arg...))
}

func (l *Logger) LogError(msg string, arg ...any) {
	(*l) = append(*l, fmt.Sprintf(msg, arg...))
}

func (l *Logger) Wrap(h http.Handler) http.Handler {
	return h
}

func TestLogger(t *testing.T) {
	var logger Logger
	var mux http.ServeMux
	server := gear.NewTestServer(&mux, &logger)
	defer server.Close()

	geartest.Curl(server.URL)

	if log := logger[0]; log != "Middleware added: *gear_test.Logger" {
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
	server := gear.NewTestServer(&mux, &logger)
	defer server.Close()

	geartest.Curl(server.URL)

	if log := logger.Logger[0]; log != "Middleware added: MyLogger" {
		t.Fatal(log)
	}
}

func TestPanicRecover(t *testing.T) {
	var logger Logger
	var mux http.ServeMux
	server := gear.NewTestServer(&mux, gear.DefaultPanicRecover, &logger)
	defer server.Close()

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		panic("some error")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gear.G(r).LogInfo("info")
	})

	geartest.Curl(path.Join(server.URL, "/error"))
	geartest.Curl(server.URL)

	if len(logger) != 3 {
		t.Fatal(logger)
	}

	if log := logger[0]; log != "Middleware added: *gear_test.Logger" {
		t.Fatal(log)
	}
	if log := logger[1]; log != "recovered from panic: some error" {
		t.Fatal(log)
	}
	if log := logger[2]; log != "info" {
		t.Fatal(log)
	}

}
