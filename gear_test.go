package gear_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mkch/gear"
	"github.com/mkch/gear/encoding"
	"github.com/mkch/gear/impl/geartest"
	"github.com/mkch/gg"
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

func TestXMLBodyDecoder(t *testing.T) {
	var respBody = "abc\ndef"
	var mux http.ServeMux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		g := gear.G(r)
		var data struct {
			N int
			S string
		}
		err := g.MustDecodeBody(&data)
		if err != nil {
			t.Log(err)
			return
		}
		if data.N != 1 || data.S != "str" {
			g.Code(http.StatusBadRequest)
			return
		}
		io.WriteString(w, respBody)
	})
	server := gear.NewTestServer(&mux)
	defer server.Close()

	body, vars := geartest.CurlPOST(server.URL, "text/xml", `<xml2> <N>1</N> <S>str</S> </xml>`, "-w", "\n%{http_code}")
	if code := vars["response_code"]; code.(float64) != float64(200) {
		t.Fatal(code)
	}
	if string(body) != respBody {
		t.Fatal(string(body))
	}
}

func TestMiddleWare(t *testing.T) {
	var logs []string
	var logMiddleware = gear.MiddlewareFuncWitName(func(g *gear.Gear, next func(*gear.Gear)) {
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

type CustomLogger []string

func (l *CustomLogger) Serve(g *gear.Gear, next func(*gear.Gear)) {
	*l = append(*l, fmt.Sprintf("Method: %v Path: %v", g.R.Method, g.R.URL.Path))
	next(g)
}

func TestCustomLogger(t *testing.T) {
	var logger CustomLogger
	var mux http.ServeMux
	server := gear.NewTestServer(&mux, &logger)
	defer server.Close()

	geartest.Curl(server.URL)

	if log := logger[0]; log != "Method: GET Path: /" {
		t.Fatal(log)
	}
}

type LoggerWithName struct {
	CustomLogger
}

func (l *LoggerWithName) MiddlewareName() string {
	return "MyLogger"
}

func TestPanicRecover(t *testing.T) {
	var logger CustomLogger
	var mux http.ServeMux

	var w = &bytes.Buffer{}
	var oldLogger = gear.RawLogger
	defer func() { gear.RawLogger = oldLogger }()
	gear.RawLogger = slog.New(slog.NewTextHandler(w,
		&slog.HandlerOptions{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == "time" {
					return slog.Attr{}
				}
				return a
			},
		}))
	server := gear.NewTestServer(&mux, gear.PanicRecovery(false), &logger)
	defer server.Close()

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		panic("some error")
	})

	geartest.Curl(server.URL + "/error")
	geartest.Curl(server.URL)

	if output := w.String(); !strings.HasSuffix(output, `level=ERROR msg="recovered from panic" value="some error"`+"\n") {
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

func TestPathInterceptor(t *testing.T) {
	var mux http.ServeMux
	handler := gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		io.WriteString(g.W, g.R.URL.Path)
		g.Stop()
	})
	server := gear.NewTestServer(&mux, gear.NewPathInterceptor("/a/b", handler))
	if _, vars := geartest.Curl(server.URL + "/a"); vars["response_code"] != float64(404) {
		t.Fatal(vars["response_code"])
	}
	if body, _ := geartest.Curl(server.URL + "/a/b"); string(body) != "/a/b" {
		t.Fatal(string(body))
	}
	if body, _ := geartest.Curl(server.URL + "/a/b/"); string(body) != "/a/b/" {
		t.Fatal(string(body))
	}
	if body, _ := geartest.Curl(server.URL + "/a/b/c"); string(body) != "/a/b/c" {
		t.Fatal(string(body))
	}
}

func TestGroup(t *testing.T) {
	var mux http.ServeMux

	gear.NewGroup("/a/b", &mux, gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		fmt.Fprintf(g.W, "group1: %v\n", g.R.URL.Path)
		next(g)
	})).Handle("/1/2", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path: %v\n", r.URL.Path)
	})).Group("c", gear.MiddlewareFunc(func(g *gear.Gear, next func(*gear.Gear)) {
		fmt.Fprintf(g.W, "group2: %v\n", g.R.URL.Path)
		next(g)
	})).Handle("/3", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path: %v\n", r.URL.Path)
	}))

	server := gear.NewTestServer(&mux)
	if _, vars := geartest.Curl(server.URL + "/a/b"); vars["response_code"] != float64(404) {
		t.Fatal(vars["response_code"])
	}
	if body, _ := geartest.Curl(server.URL + "/a/b/1/2"); string(body) != "group1: /a/b/1/2\npath: /a/b/1/2\n" {
		t.Fatal(string(body))
	}
	if body, _ := geartest.Curl(server.URL + "/a/b/c/3"); string(body) != "group1: /a/b/c/3\ngroup2: /a/b/c/3\npath: /a/b/c/3\n" {
		t.Fatal(string(body))
	}
}

func TestGStop(t *testing.T) {
	var h1Run bool
	h1 := gear.WrapFunc(func(w http.ResponseWriter, r *http.Request) {
		h1Run = true
	})

	h2 := gear.WrapFunc(func(w http.ResponseWriter, r *http.Request) {
		gear.G(r).Stop()
		h1.ServeHTTP(w, r)
	})

	h2.ServeHTTP(nil, gg.Must(http.NewRequest(http.MethodGet, "/", nil)))

	if h1Run {
		t.Fatal("h1 should not run")
	}
}

func TestDecodeForm(t *testing.T) {
	type Person struct {
		Name    string   `map:"name"`
		Age     int16    `map:"age"`
		Hobbies []string `map:"hobby"`
	}

	var person Person

	var mux http.ServeMux

	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		g := gear.G(r)
		gear.LogIfErr(g.MustDecodeForm(&person))
	})
	server := gear.NewTestServer(&mux)
	defer server.Close()

	_, vars := geartest.Curl(server.URL+"/post?hobby=football", "-d", "name=John&age=30&hobby=basketball")
	if status := int(vars["response_code"].(float64)); status != 200 {
		t.Fatal(status)
	}
	slices.Sort(person.Hobbies)
	if !reflect.DeepEqual(person, Person{
		Name: "John", Age: 30, Hobbies: []string{"basketball", "football"},
	}) {
		t.Fatal(person)
	}
}

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

func TestDecodeFormMultipart(t *testing.T) {
	type Person struct {
		Name    *Name    `map:"name"`
		Age     int16    `map:"age"`
		Hobbies []string `map:"hobby"`
	}

	var person Person

	var mux http.ServeMux

	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		g := gear.G(r)
		gear.LogIfErr(r.ParseMultipartForm(1024))
		gear.LogIfErr(g.MustDecodeForm(&person))
	})
	server := gear.NewTestServer(&mux)
	defer server.Close()

	_, vars := geartest.Curl(server.URL+"/post?hobby=football", "-F", "name=John Smith", "-F", "age=30", "-F", "hobby=basketball")
	if status := int(vars["response_code"].(float64)); status != 200 {
		t.Fatal(status)
	}
	slices.Sort(person.Hobbies)
	if !reflect.DeepEqual(person, Person{
		Name: &Name{"John", "Smith"}, Age: 30, Hobbies: []string{"basketball", "football"},
	}) {
		t.Fatal(person)
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	withLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a = slog.Attr{}
			}
			return a
		},
	})), func() {
		var mux http.ServeMux
		server := gear.NewTestServer(&mux, gear.Logger(&gear.LoggerOptions{
			Keys: map[string]bool{
				gear.LoggerMethodKey: true,
				gear.LoggerURLKey:    true,
				gear.LoggerHeaderKey: true},
			HeaderKeys: []string{"X-My-Header", "User-Agent"}}))
		defer server.Close()
		geartest.Curl(server.URL+"/a/b/c?x=y", "-H", "X-My-Header: v1", "-H", "User-Agent: test/1")
		expected := `level=INFO msg=HTTP method=GET URL="/a/b/c?x=y" header.X-My-Header=[v1] header.User-Agent=[test/1]` + "\n"
		if line := buf.String(); line != expected {
			t.Fatal(line)
		}
	})
}

func TestDecodeHeader(t *testing.T) {
	var mux http.ServeMux
	type Header struct {
		IfModifiedSince encoding.HTTPDate `map:"If-Modified-Since"`
		UserAgent       string            `map:"User-Agent"`
	}
	var header Header
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var g = gear.G(r)
		if err := g.DecodeHeader(&header); err != nil {
			t.Fatal(err)
		}
	})
	server := gear.NewTestServer(&mux)
	defer server.Close()
	geartest.Curl(server.URL, "-H", "User-Agent: test/1", "-H", "If-Modified-Since: "+strings.Replace(time.Now().In(time.UTC).Format(time.RFC1123), " UTC", " GMT", 1))
	if header.UserAgent != "test/1" {
		t.Fatal(header)
	}
	if since := time.Since(time.Time(header.IfModifiedSince)); since > time.Second || since < 0 {
		t.Fatal(header)
	}
}

func TestDecodeQuery(t *testing.T) {
	var mux http.ServeMux
	type User struct {
		Username string `map:"user"`
	}
	var user User
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var g = gear.G(r)
		if err := g.DecodeQuery(&user); err != nil {
			t.Fatal(err)
		}
	})
	server := gear.NewTestServer(&mux)
	defer server.Close()
	geartest.Curl(server.URL + "/?user=abc&id=100")
	if user.Username != "abc" {
		t.Fatal(user)
	}
}
