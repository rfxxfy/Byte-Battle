package problems

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestProblem(t *testing.T, base, slug, version, manifest string, tests map[string][2]string) {
	t.Helper()
	dir := filepath.Join(base, slug, version)
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "reference"), 0o755); err != nil {
		t.Fatalf("mkdir reference: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "statement.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatalf("write statement.md: %v", err)
	}
	for name, pair := range tests {
		if err := os.WriteFile(filepath.Join(dir, "tests", name+".in"), []byte(pair[0]), 0o644); err != nil {
			t.Fatalf("write %s.in: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "tests", name+".out"), []byte(pair[1]), 0o644); err != nil {
			t.Fatalf("write %s.out: %v", name, err)
		}
	}
}

var sampleManifest = `{"title":"Add","difficulty":"easy","time_limit_ms":1000,"memory_limit_mb":256}`

func TestStore_GetByPath_OK(t *testing.T) {
	dir := t.TempDir()
	writeTestProblem(t, dir, "001-add", "v1", sampleManifest, map[string][2]string{
		"01": {"1 2\n", "3\n"},
		"02": {"0 0\n", "0\n"},
	})

	s := NewStore(dir)
	p, err := s.GetByPath("001-add/v1")
	if err != nil {
		t.Fatalf("GetByPath: %v", err)
	}
	if p.Manifest.Title != "Add" {
		t.Errorf("title = %q", p.Manifest.Title)
	}
	if p.Slug != "001-add" {
		t.Errorf("slug = %q", p.Slug)
	}
	if len(p.TestCases) != 2 {
		t.Errorf("test cases = %d", len(p.TestCases))
	}
}

func TestStore_GetByPath_Cached(t *testing.T) {
	dir := t.TempDir()
	writeTestProblem(t, dir, "001-add", "v1", sampleManifest, map[string][2]string{
		"01": {"1\n", "1\n"},
	})
	s := NewStore(dir)
	p1, _ := s.GetByPath("001-add/v1")
	p2, _ := s.GetByPath("001-add/v1")
	if p1 != p2 {
		t.Error("expected cached pointer")
	}
}

func TestStore_GetByPath_NotFound(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.GetByPath("nope/v1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStore_GetByPath_NoTests(t *testing.T) {
	dir := t.TempDir()
	vDir := filepath.Join(dir, "001-x", "v1")
	if err := os.MkdirAll(vDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vDir, "manifest.json"), []byte(sampleManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vDir, "statement.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	_, err := s.GetByPath("001-x/v1")
	if err == nil {
		t.Fatal("expected error for no test cases")
	}
}

func TestNormalizeAndMatch(t *testing.T) {
	if NormalizeOutput("hello  \n\n") != "hello" {
		t.Error("normalize failed")
	}
	if !Match("3\n", "3\n\n") {
		t.Error("should match")
	}
	if Match("3", "4") {
		t.Error("should not match")
	}
}
