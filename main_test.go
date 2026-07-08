package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunResolveUnpublishedSupplemental(t *testing.T) {
	inputDir := filepath.Join(t.TempDir(), "input_data")
	if err := os.Mkdir(inputDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	files := map[string]string{
		"target.csv": "id,title\n100,One\n101,Two\n",
		"rollback.csv": strings.Join([]string{
			"# generated",
			"# config",
			"# input",
			"2000",
			"2001",
			"",
		}, "\n"),
		"target.unpublished_supplemental.csv": strings.Join([]string{
			"id,node_id,file,media_use_tid,published",
			"100,,/mnt/islandora_staging/private.pdf,151326,0",
			"101,,/mnt/islandora_staging/private2.pdf,151326,0",
			"",
		}, "\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(inputDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	if err := runResolveUnpublishedSupplemental(inputDir); err != nil {
		t.Fatalf("unexpected resolver error: %v", err)
	}

	addMedia, err := os.ReadFile(filepath.Join(inputDir, "target.add_media.csv"))
	if err != nil {
		t.Fatalf("failed to read target.add_media.csv: %v", err)
	}
	got := strings.ReplaceAll(string(addMedia), "\r\n", "\n")
	expected := strings.Join([]string{
		"node_id,file,media_use_tid,published",
		"2000,/mnt/islandora_staging/private.pdf,151326,0",
		"2001,/mnt/islandora_staging/private2.pdf,151326,0",
		"",
	}, "\n")
	if got != expected {
		t.Fatalf("unexpected add_media CSV:\n%s", got)
	}

	if _, err := os.Stat(filepath.Join(inputDir, "target.unpublished_supplemental.csv")); !os.IsNotExist(err) {
		t.Fatalf("expected pending unpublished supplemental CSV to be removed, got err %v", err)
	}
}
