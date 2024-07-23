package geartest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
)

var responseRegexp = regexp.MustCompile(`(?s)(.*)\n(.+)`)

// Curl initiates a curl call.
// url is the URL to request.
// params are curl command line params.
// respBody is the response body returned.
// vars are the json format of all curl write-out variables.
// See https://everything.curl.dev/usingcurl/verbose/writeout.html
func Curl(url string, params ...string) (respBody []byte, vars map[string]any) {
	cmd := exec.Command("curl", append(params, []string{
		"-w", "\n%{json}",
		"--no-progress-meter",
		url}...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("curl: %w", err))
	}
	if str := stderr.String(); len(str) != 0 {
		panic(fmt.Errorf("curl errored out: %s", str))
	}
	m := responseRegexp.FindSubmatch(stdout.Bytes())
	respBody = m[1]
	if err := json.Unmarshal(m[2], &vars); err != nil {
		panic(fmt.Errorf("curl: %w", err))
	}
	return
}

// CurlPOST initiates a curl POST request.
// See [Curl] for details.
func CurlPOST(url string, contentType string, data string, params ...string) (respBody []byte, vars map[string]any) {
	return Curl(url, append([]string{"-H", "Content-Type: " + contentType, "-d", data}, params...)...)
}
