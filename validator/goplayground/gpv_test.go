package goplayground_test

import (
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/mkch/gear"
	"github.com/mkch/gear/encoding"
	"github.com/mkch/gear/internal/geartest"
	_ "github.com/mkch/gear/validator/goplayground"
)

func TestValidator(t *testing.T) {
	type User struct {
		ID   int `validate:"required"`
		Name string
	}
	var err atomic.Pointer[error]
	http.HandleFunc("/user_ok", func(w http.ResponseWriter, r *http.Request) {
		var user User
		e := gear.G(r).DecodeBody(&user)
		err.Store(&e)
	})
	http.HandleFunc("/user_no_id", func(w http.ResponseWriter, r *http.Request) {
		var user User
		e := gear.G(r).DecodeBody(&user)
		err.Store(&e)
	})
	http.HandleFunc("/user_str", func(w http.ResponseWriter, r *http.Request) {
		var user string
		e := gear.G(r).DecodeBody(&user)
		err.Store(&e)
	})
	server := gear.NewTestServer(nil)
	defer server.Close()
	geartest.CurlPOST(server.URL+"/user_ok", encoding.MIME_JSON, `{"ID":1,"Name":"User1"}`, "-w", "\n%{http_code}")
	if e := *err.Load(); e != nil {
		panic(e)
	}
	geartest.CurlPOST(server.URL+"/user_no_id", encoding.MIME_JSON, `{"Name":"User1"}`, "-w", "\n%{http_code}")
	if e := *err.Load(); e == nil {
		panic("should be an invalidation error, no id")
	}
	geartest.CurlPOST(server.URL+"/user_str", encoding.MIME_JSON, `"User1"`, "-w", "\n%{http_code}")
	if e := *err.Load(); e != nil {
		panic(e)
	}
}
