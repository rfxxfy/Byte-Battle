package problems

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ProblemMeta struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
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
	ID        string
	Meta      ProblemMeta
	TestCases []TestCase
}

type Loader struct {
	problems map[string]*Problem
	order    []string
}

func NewLoader(dir string) (*Loader, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("problems directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("problems path %q is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading problems directory: %w", err)
	}

	l := &Loader{problems: make(map[string]*Problem)}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		p, err := loadProblem(id, filepath.Join(dir, id))
		if err != nil {
			return nil, fmt.Errorf("loading problem %q: %w", id, err)
		}
		l.problems[id] = p
		l.order = append(l.order, id)
	}

	if len(l.problems) == 0 {
		return nil, fmt.Errorf("no problems found in %q", dir)
	}

	hasTests := false
	for _, p := range l.problems {
		if len(p.TestCases) > 0 {
			hasTests = true
			break
		}
	}
	if !hasTests {
		return nil, fmt.Errorf("no problem in %q contains test cases", dir)
	}

	sort.Strings(l.order)
	return l, nil
}

func (l *Loader) Get(id string) (*Problem, error) {
	p, ok := l.problems[id]
	if !ok {
		return nil, fmt.Errorf("problem %q not found", id)
	}
	return p, nil
}

func (l *Loader) List() []*Problem {
	out := make([]*Problem, 0, len(l.order))
	for _, id := range l.order {
		out = append(out, l.problems[id])
	}
	return out
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

func loadProblem(id, dir string) (*Problem, error) {
	data, err := os.ReadFile(filepath.Join(dir, "problem.json"))
	if err != nil {
		return nil, fmt.Errorf("reading problem.json: %w", err)
	}
	var meta ProblemMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing problem.json: %w", err)
	}
	tests, err := loadTestCases(filepath.Join(dir, "tests"))
	if err != nil {
		return nil, fmt.Errorf("loading tests: %w", err)
	}
	return &Problem{ID: id, Meta: meta, TestCases: tests}, nil
}

func loadTestCases(dir string) ([]TestCase, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
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
		var expectedData []byte
		outPath := filepath.Join(dir, base+".out")
		if _, serr := os.Stat(outPath); serr == nil {
			expectedData, _ = os.ReadFile(outPath)
		}
		cases = append(cases, TestCase{
			Name:     base,
			Input:    string(inputData),
			Expected: string(expectedData),
		})
	}
	return cases, nil
}
