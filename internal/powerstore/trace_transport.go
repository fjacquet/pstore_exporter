package powerstore

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// tracingRoundTripper logs method, URL, status, and the response BODY of every
// request that flows through it, so payload shapes can be validated against a
// live array (`--trace`). Request and response headers are NEVER logged:
// PowerStore authenticates with HTTP Basic credentials and a DELL-EMC-TOKEN
// CSRF header, and both travel in headers. Responses to /login_session are
// skipped entirely (session material). Verbose — debugging only.
//
// Scope: gopowerstore (v1.22.0) builds its *http.Client inside api.New with no
// ClientOptions seam to inject a transport, so this wrapper can only cover the
// repo-owned raw HTTP path (the bulk latest_five_min_metrics endpoints).
type tracingRoundTripper struct {
	array string
	next  http.RoundTripper
}

// newTracingRoundTripper wraps next (http.DefaultTransport when nil).
func newTracingRoundTripper(array string, next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return &tracingRoundTripper{array: array, next: next}
}

// RoundTrip forwards the request and logs the response status and body. The
// body is read in full and replaced, so the caller still sees an intact stream.
func (t *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	if strings.HasSuffix(req.URL.Path, "/login_session") {
		return resp, nil // login responses carry session material; never trace them
	}

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("trace: read response body: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("trace: close response body: %w", closeErr)
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	// URL.Redacted() masks any password embedded in the URL itself; auth set
	// via headers (the normal path) never appears in a URL anyway.
	entry := log.WithFields(log.Fields{
		"array":  t.array,
		"method": req.Method,
		"url":    req.URL.Redacted(),
		"status": resp.StatusCode,
	})
	if printableBody(resp.Header.Get("Content-Type")) {
		entry.Infof("API trace:\n%s", body)
	} else {
		// The bulk download endpoint returns a gzipped tar; raw bytes would be
		// log noise, so only the size is recorded.
		entry.Infof("API trace: non-text body elided (%d bytes)", len(body))
	}
	return resp, nil
}

// printableBody reports whether a response body is text-like and useful to log
// verbatim.
func printableBody(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "json") || strings.Contains(ct, "xml") || strings.HasPrefix(ct, "text/")
}
