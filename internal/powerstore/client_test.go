package powerstore

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dell/gopowerstore"
	"github.com/dell/gopowerstore/api"
)

// TestParallelSamplesRespectsConcurrencyLimit is the regression guard for the
// fan-out cap: with more items than the limit, no more than `limit` callbacks
// may run at once, and the per-item samples must still come back in item order.
func TestParallelSamplesRespectsConcurrencyLimit(t *testing.T) {
	const limit = 3
	items := make([]int, 12)
	for i := range items {
		items[i] = i
	}

	var mu sync.Mutex
	var cur, max int
	fn := func(_ context.Context, i int) []Sample {
		mu.Lock()
		cur++
		if cur > max {
			max = cur
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // hold the slot so overlap is observable
		mu.Lock()
		cur--
		mu.Unlock()
		return []Sample{{Name: "x", Value: float64(i)}}
	}

	got := parallelSamples(context.Background(), items, limit, fn)

	if max == 0 {
		t.Fatal("callback never ran")
	}
	if max > limit {
		t.Fatalf("max concurrent callbacks %d exceeded limit %d", max, limit)
	}
	if len(got) != len(items) {
		t.Fatalf("want %d samples, got %d", len(items), len(got))
	}
	for i := range got {
		if got[i].Value != float64(i) {
			t.Fatalf("sample %d out of order: got value %v", i, got[i].Value)
		}
	}
}

// TestCollectActiveAlertsFiltersToActiveState asserts the fetch filters
// server-side to ACTIVE alerts, so the result set stays small regardless of how
// many historical (CLEARED) alerts the array has accumulated.
func TestCollectActiveAlertsFiltersToActiveState(t *testing.T) {
	var got gopowerstore.GetAlertsOpts
	get := func(_ context.Context, opts gopowerstore.GetAlertsOpts) (*gopowerstore.GetAlertsResponse, error) {
		got = opts
		return &gopowerstore.GetAlertsResponse{Alerts: gopowerstore.Alerts{{ID: "a", Severity: "Major", State: "ACTIVE"}}}, nil
	}
	alerts, err := collectActiveAlerts(context.Background(), get, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Queries["state"] != "eq.ACTIVE" {
		t.Fatalf("expected server-side state=eq.ACTIVE filter, got Queries=%v", got.Queries)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

// TestCollectActiveAlertsPaginatesUntilShortPage is the core regression guard for
// the original bug: the exporter fetched only the first server-default page of
// alerts. The fetch must page through every full page and stop on the first
// short page, accumulating all alerts.
func TestCollectActiveAlertsPaginatesUntilShortPage(t *testing.T) {
	pages := []gopowerstore.Alerts{
		{{ID: "1"}, {ID: "2"}}, // full page (== pageSize) -> keep paging
		{{ID: "3"}},            // short page (< pageSize) -> stop
	}
	var starts []int
	call := 0
	get := func(_ context.Context, opts gopowerstore.GetAlertsOpts) (*gopowerstore.GetAlertsResponse, error) {
		starts = append(starts, opts.StartIndex)
		p := pages[call]
		call++
		return &gopowerstore.GetAlertsResponse{Alerts: p}, nil
	}
	alerts, err := collectActiveAlerts(context.Background(), get, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if call != 2 {
		t.Fatalf("expected 2 page fetches, got %d", call)
	}
	if len(alerts) != 3 {
		t.Fatalf("expected 3 accumulated alerts, got %d", len(alerts))
	}
	if len(starts) != 2 || starts[0] != 0 || starts[1] != 2 {
		t.Fatalf("expected StartIndex progression [0 2], got %v", starts)
	}
}

// TestCollectActiveAlertsPropagatesError ensures a page-fetch failure surfaces to
// the caller (which logs and degrades gracefully) rather than returning a
// silently truncated alert set.
func TestCollectActiveAlertsPropagatesError(t *testing.T) {
	get := func(_ context.Context, _ gopowerstore.GetAlertsOpts) (*gopowerstore.GetAlertsResponse, error) {
		return nil, errors.New("boom")
	}
	if _, err := collectActiveAlerts(context.Background(), get, 2); err == nil {
		t.Fatal("expected page-fetch error to propagate")
	}
}

// TestIsNotFound asserts that only a 404 API error is treated as the expected
// "resource absent" condition. Real failures — server errors, timeouts — must
// NOT be classified as not-found, so they still produce a warning.
func TestIsNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"404 APIError is not-found", gopowerstore.APIError{ErrorMsg: &api.ErrorMsg{StatusCode: http.StatusNotFound}}, true},
		{"500 APIError is not not-found", gopowerstore.APIError{ErrorMsg: &api.ErrorMsg{StatusCode: http.StatusInternalServerError}}, false},
		{"deadline exceeded still warns", context.DeadlineExceeded, false},
		{"generic error still warns", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotFound(tc.err); got != tc.want {
				t.Fatalf("isNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
