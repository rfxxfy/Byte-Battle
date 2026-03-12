package problems

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestProblem(t *testing.T, base, id, meta string, tests map[string][2]string) {
	t.Helper()
	dir := filepath.Join(base, id)
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0o755); err != nil {
		t.Fatalf("mkdir tests dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "problem.json"), []byte(meta), 0o644); err != nil {
		t.Fatalf("write problem.json: %v", err)
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

var sampleMeta = `{"title":"Add","description":"a+b","difficulty":"easy","time_limit_ms":1000,"memory_limit_mb":256}`

func TestNewLoader_OK(t *testing.T) {
	dir := t.TempDir()
	writeTestProblem(t, dir, "001-add", sampleMeta, map[string][2]string{
		"01": {"1 2\n", "3\n"},
		"02": {"0 0\n", "0\n"},
	})
	l, err := NewLoader(dir)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	if len(l.List()) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(l.List()))
	}
	p, err := l.Get("001-add")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.Meta.Title != "Add" {
		t.Errorf("title = %q", p.Meta.Title)
	}
	if len(p.TestCases) != 2 {
		t.Errorf("test cases = %d", len(p.TestCases))
	}
}

func TestNewLoader_EmptyDir(t *testing.T) {
	_, err := NewLoader(t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewLoader_NoTests(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, "001-x")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir problem dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(d, "problem.json"), []byte(sampleMeta), 0o644); err != nil {
		t.Fatalf("write problem.json: %v", err)
	}
	_, err := NewLoader(dir)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewLoader_FailsIfAnyProblemHasNoTests(t *testing.T) {
	dir := t.TempDir()
	writeTestProblem(t, dir, "001-ok", sampleMeta, map[string][2]string{"01": {"1\n", "1\n"}})

	d := filepath.Join(dir, "002-empty")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir problem dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(d, "problem.json"), []byte(sampleMeta), 0o644); err != nil {
		t.Fatalf("write problem.json: %v", err)
	}

	_, err := NewLoader(dir)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGet_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestProblem(t, dir, "001-y", sampleMeta, map[string][2]string{"01": {"1\n", "1\n"}})
	l, err := NewLoader(dir)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	_, err = l.Get("nope")
	if err == nil {
		t.Fatal("expected error")
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
