package powerstore

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func makeGzTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseBulkArchive(t *testing.T) {
	csv := "appliance_id,read_iops,write_iops\nappl-1,100,50\n"
	archive := makeGzTar(t, map[string]string{"performance_metrics_by_appliance.csv": csv})
	files, err := parseBulkArchive(archive)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rows := files["performance_metrics_by_appliance.csv"]
	if len(rows) != 1 || rows[0]["read_iops"] != "100" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
