package powerstore

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

// captureLogs redirects the global logrus output to a buffer for one test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := log.StandardLogger().Out
	origLevel := log.GetLevel()
	origFormatter := log.StandardLogger().Formatter
	log.SetOutput(&buf)
	log.SetLevel(log.InfoLevel)
	// DisableQuote keeps the logged body verbatim so assertions can match it.
	log.SetFormatter(&log.TextFormatter{DisableQuote: true})
	t.Cleanup(func() {
		log.SetOutput(orig)
		log.SetLevel(origLevel)
		log.SetFormatter(origFormatter)
	})
	return &buf
}

func traceGet(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()
	client := &http.Client{Transport: newTracingRoundTripper("test-array", nil)}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
	return resp, body
}

// Fixed payloads served by the test handlers below.
const (
	testJSONBody   = `{"id":"A1","name":"appliance-1"}`
	testLoginBody  = `[{"id":"session-1"}]`
	testSecretTok  = "super-secret-csrf-token"
	testBinaryBody = "\x1f\x8b\x08\x00\xde\xad\xbe\xef" // gzip magic + filler
)

// TestTracingRoundTripperLogsBodyNotHeaders asserts the token-safety contract:
// method, URL, status, and body are logged; response headers (which carry the
// DELL-EMC-TOKEN CSRF token) never are. The body must remain fully readable by
// the caller after being traced.
func TestTracingRoundTripperLogsBodyNotHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("DELL-EMC-TOKEN", testSecretTok)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testJSONBody))
	}))
	defer srv.Close()

	buf := captureLogs(t)
	resp, body := traceGet(t, srv.URL+"/api/rest/appliance")

	if string(body) != testJSONBody {
		t.Errorf("caller body corrupted by trace: got %q, want %q", body, testJSONBody)
	}
	logged := buf.String()
	for _, want := range []string{"GET", srv.URL + "/api/rest/appliance", "200", testJSONBody, "test-array"} {
		if !strings.Contains(logged, want) {
			t.Errorf("trace log missing %q; log: %s", want, logged)
		}
	}
	if strings.Contains(logged, testSecretTok) {
		t.Errorf("trace log leaked the DELL-EMC-TOKEN header value; log: %s", logged)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

// TestTracingRoundTripperSkipsLoginSession asserts that responses to the
// login_session endpoint are never traced (they establish session material).
func TestTracingRoundTripperSkipsLoginSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testLoginBody))
	}))
	defer srv.Close()

	buf := captureLogs(t)
	_, body := traceGet(t, srv.URL+"/api/rest/login_session")

	if string(body) != testLoginBody {
		t.Errorf("caller body corrupted: got %q, want %q", body, testLoginBody)
	}
	if logged := buf.String(); strings.Contains(logged, "API trace") || strings.Contains(logged, "session-1") {
		t.Errorf("login_session response must not be traced; log: %s", logged)
	}
}

// TestTracingRoundTripperElidesBinaryBody asserts that non-text payloads (the
// bulk download is a gzipped tar) are logged as a size, not raw bytes.
func TestTracingRoundTripperElidesBinaryBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(testBinaryBody))
	}))
	defer srv.Close()

	buf := captureLogs(t)
	_, body := traceGet(t, srv.URL+"/api/rest/metrics/latest_five_min_metrics/download")

	if string(body) != testBinaryBody {
		t.Errorf("caller body corrupted: got %v, want %v", body, []byte(testBinaryBody))
	}
	logged := buf.String()
	if !strings.Contains(logged, "non-text body elided (8 bytes)") {
		t.Errorf("trace log should record the binary body size; log: %s", logged)
	}
	if strings.Contains(logged, testBinaryBody) {
		t.Errorf("trace log must not contain raw binary bytes; log: %s", logged)
	}
}
