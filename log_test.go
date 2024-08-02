package gear_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/mkch/gear"
	runtimegg "github.com/mkch/gg/runtime"
)

func withLogger(logger *slog.Logger, f func()) {
	old := gear.RawLogger
	gear.RawLogger = logger
	defer func() { gear.RawLogger = old }()
	f()
}

func TestLog(t *testing.T) {
	var w = &bytes.Buffer{}
	var level slog.LevelVar // Default to LevelInfo.
	type msg struct {
		Level  string      `json:"level"`
		Msg    string      `json:"msg"`
		Source slog.Source `json:"source"`
		Val    int         `json:"val"`
		Ret    string      `json:"ret"`
		Err    string      `json:"err"`
	}
	withLogger(slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{AddSource: true, Level: &level})), func() {
		srcD := runtimegg.Source()
		gear.LogD("debug", "val", 1)
		srcD.Line--
		gear.LogI("info", "val", 2)
		srcI := runtimegg.Source()
		srcI.Line--
		gear.LogW("warn", "val", 3)
		srcW := runtimegg.Source()
		srcW.Line--
		gear.LogE("error", "val", 4)
		srcE := runtimegg.Source()
		srcE.Line--
		gear.LogIfErr(errors.New("err"))
		srcIE := runtimegg.Source()
		srcIE.Line--
		gear.LogIfErrT("ret", errors.New("err2"))
		srcIET := runtimegg.Source()
		srcIET.Line--

		var m msg
		decoder := json.NewDecoder(w)
		if err := decoder.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if m != (msg{Level: "INFO", Msg: "info", Source: srcI, Val: 2}) {
			t.Fatal(m)
		}

		m = msg{}
		if err := decoder.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if m != (msg{Level: "WARN", Msg: "warn", Source: srcW, Val: 3}) {
			t.Fatal(m)
		}

		m = msg{}
		if err := decoder.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if m != (msg{Level: "ERROR", Msg: "error", Source: srcE, Val: 4}) {
			t.Fatal(m)
		}

		m = msg{}
		if err := decoder.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if m != (msg{Level: "ERROR", Msg: "", Source: srcIE, Err: "err"}) {
			t.Fatal(m)
		}

		m = msg{}
		if err := decoder.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if m != (msg{Level: "ERROR", Msg: "", Source: srcIET, Ret: "ret", Err: "err2"}) {
			t.Fatal(m)
		}
	})
}
