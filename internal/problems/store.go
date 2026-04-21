package problems

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Manifest struct {
	Slug          string `json:"slug,omitempty"`
	Title         string `json:"title"`
	Difficulty    string `json:"difficulty"`
	TimeLimitMs   int    `json:"time_limit_ms"`
	MemoryLimitMb int    `json:"memory_limit_mb"`
}

type TestCase struct {
	Name     string
	Input    string
	Expected string
}

type Problem struct {
	Slug      string
	Statement string
	Manifest  Manifest
	TestCases []TestCase
}

type Store struct {
	baseDir string
	mu      sync.RWMutex
	cache   map[string]*Problem
}

func NewStore(baseDir string) *Store {
	return &Store{
		baseDir: baseDir,
		cache:   make(map[string]*Problem),
	}
}

func (s *Store) GetByPath(artifactPath string) (*Problem, error) {
	s.mu.RLock()
	p, ok := s.cache[artifactPath]
	s.mu.RUnlock()
	if ok {
		return p, nil
	}

	p, err := loadProblemFromDir(artifactPath, filepath.Join(s.baseDir, filepath.FromSlash(artifactPath)))
	if err != nil {
		return nil, fmt.Errorf("load problem %q: %w", artifactPath, err)
	}

	s.mu.Lock()
	s.cache[artifactPath] = p
	s.mu.Unlock()
	return p, nil
}

func loadProblemFromDir(artifactPath, dir string) (*Problem, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("reading manifest.json: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest.json: %w", err)
	}
	if manifest.Title == "" {
		return nil, fmt.Errorf("manifest.json: title is required")
	}

	stmtBytes, err := os.ReadFile(filepath.Join(dir, "statement.md"))
	if err != nil {
		return nil, fmt.Errorf("reading statement.md: %w", err)
	}

	tests, err := loadTestCases(filepath.Join(dir, "tests"))
	if err != nil {
		return nil, fmt.Errorf("loading tests: %w", err)
	}
	if len(tests) == 0 {
		return nil, fmt.Errorf("problem has no test cases")
	}

	slug := strings.SplitN(artifactPath, "/", 2)[0]

	return &Problem{
		Slug:      slug,
		Statement: string(stmtBytes),
		Manifest:  manifest,
		TestCases: tests,
	}, nil
}

func loadTestCases(dir string) ([]TestCase, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var inNames []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".in") {
			inNames = append(inNames, e.Name())
		}
	}
	sort.Strings(inNames)

	var cases []TestCase
	for _, inFile := range inNames {
		base := strings.TrimSuffix(inFile, ".in")
		inputData, err := os.ReadFile(filepath.Join(dir, inFile))
		if err != nil {
			return nil, err
		}
		expectedData, err := os.ReadFile(filepath.Join(dir, base+".out"))
		if err != nil {
			return nil, fmt.Errorf("reading %s.out: %w", base, err)
		}
		cases = append(cases, TestCase{
			Name:     base,
			Input:    string(inputData),
			Expected: string(expectedData),
		})
	}
	return cases, nil
}

func NormalizeOutput(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func Match(actual, expected string) bool {
	return NormalizeOutput(actual) == NormalizeOutput(expected)
}
