package powerstore

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// bulkHTTPTimeout bounds the enable+download round trips to the array.
const bulkHTTPTimeout = 90 * time.Second

// downloadBulkArchive enables and downloads the PowerStore "latest five minute
// metrics" bulk export, returning the raw gzipped-tar bytes.
//
// The flow mirrors the Dell reference implementation
// powerstore-metrics-exporter/collector/bulkClient/api_bulk.go
// (BulkEnable + DownloadBulkData):
//
//  1. POST <endpoint>/latest_five_min_metrics/enable   (HTTP Basic auth,
//     header DELL-VISIBILITY: Partner) — expects 204 No Content; 4xx returns
//     the localized error message.
//  2. POST <endpoint>/latest_five_min_metrics/download (HTTP Basic auth,
//     headers DELL-VISIBILITY: Partner and If-None-Match: start) — the response
//     body is a gzipped tar of CSV files.
//
// gopowerstore's typed client JSON-decodes responses, so the bulk endpoints
// (which return a binary archive) are reached with a dedicated *http.Client.
func (c *ArrayClient) downloadBulkArchive(ctx context.Context) ([]byte, error) {
	base := strings.TrimRight(c.endpoint, "/") + "/"
	httpClient := &http.Client{
		Timeout: bulkHTTPTimeout,
		Transport: &http.Transport{
			// PowerStore arrays typically present self-signed certificates.
			// Verification is governed by the operator-set per-array
			// InsecureSkipVerify config (logged at startup), not hardcoded.
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: c.insecure,
				MinVersion:         tls.VersionTLS12,
			},
		},
	}

	if err := c.bulkEnable(ctx, httpClient, base); err != nil {
		return nil, err
	}
	return c.bulkDownload(ctx, httpClient, base)
}

// bulkEnable turns on the bulk five-minute metrics export.
func (c *ArrayClient) bulkEnable(ctx context.Context, httpClient *http.Client, base string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"latest_five_min_metrics/enable", nil)
	if err != nil {
		return fmt.Errorf("array %q: build bulk enable request: %w", c.name, err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("DELL-VISIBILITY", "Partner")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("array %q: bulk enable request: %w", c.name, err)
	}
	defer resp.Body.Close()

	// 204 No Content is success; some firmware may return 200/201.
	if resp.StatusCode == http.StatusNoContent ||
		resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusCreated {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("array %q: bulk enable failed (status %d): %s", c.name, resp.StatusCode, strings.TrimSpace(string(body)))
}

// bulkDownload fetches the gzipped-tar archive of CSV metric files.
func (c *ArrayClient) bulkDownload(ctx context.Context, httpClient *http.Client, base string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"latest_five_min_metrics/download", nil)
	if err != nil {
		return nil, fmt.Errorf("array %q: build bulk download request: %w", c.name, err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("DELL-VISIBILITY", "Partner")
	req.Header.Set("If-None-Match", "start")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("array %q: bulk download request: %w", c.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("array %q: bulk download failed (status %d): %s", c.name, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("array %q: read bulk archive: %w", c.name, err)
	}
	return archive, nil
}

// parseBulkArchive decompresses a gzipped tar of CSV files into a map of
// filename -> rows, where each row is a column→value map keyed by CSV header.
func parseBulkArchive(archive []byte) (map[string][]map[string]string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	out := make(map[string][]map[string]string)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		rows, err := parseCSV(tr)
		if err != nil {
			return nil, fmt.Errorf("csv %s: %w", hdr.Name, err)
		}
		out[baseName(hdr.Name)] = rows
	}
	return out, nil
}

func parseCSV(r io.Reader) ([]map[string]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 1 {
		return nil, nil
	}
	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(rec) {
				row[col] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
