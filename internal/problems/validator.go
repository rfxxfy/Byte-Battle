package problems

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bytebattle/internal/executor"
)

const (
	MaxArchiveBytes       = 50 * 1024 * 1024  // 50 MB
	MaxExtractedBytes     = 200 * 1024 * 1024 // 200 MB
	MaxTestCases          = 100
	MaxProblemsPerUser    = 20
	MaxVersionsPerProblem = 10
)

type ValidatedProblem struct {
	Manifest      Manifest
	Statement     string
	ReferenceCode string
	ReferenceLang string
	TestCases     []TestCase
	Dir           string // temp directory; caller must os.RemoveAll when done
}

func ValidateArchive(ctx context.Context, r io.ReadSeeker, size int64, exec executor.Executor) ([]*ValidatedProblem, error) {
	if !exec.IsReady() {
		return nil, fmt.Errorf("executor not ready")
	}

	if size > MaxArchiveBytes {
		return nil, fmt.Errorf("archive too large: %d bytes (max %d)", size, MaxArchiveBytes)
	}

	tmpRoot, err := os.MkdirTemp("", "bb-upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	if err := extractArchive(r, size, tmpRoot); err != nil {
		os.RemoveAll(tmpRoot)
		return nil, fmt.Errorf("extracting archive: %w", err)
	}

	rootManifest := filepath.Join(tmpRoot, "manifest.json")
	if _, err := os.Stat(rootManifest); err == nil {
		vp, err := validateProblemDir(ctx, tmpRoot, exec)
		if err != nil {
			os.RemoveAll(tmpRoot)
			return nil, err
		}
		return []*ValidatedProblem{vp}, nil
	}

	entries, err := os.ReadDir(tmpRoot)
	if err != nil {
		os.RemoveAll(tmpRoot)
		return nil, fmt.Errorf("reading archive root: %w", err)
	}

	var results []*ValidatedProblem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(tmpRoot, e.Name())
		if _, err := os.Stat(filepath.Join(subDir, "manifest.json")); err != nil {
			continue
		}
		vp, err := validateProblemDir(ctx, subDir, exec)
		if err != nil {
			os.RemoveAll(tmpRoot)
			return nil, fmt.Errorf("problem %q: %w", e.Name(), err)
		}
		results = append(results, vp)
	}

	if len(results) == 0 {
		os.RemoveAll(tmpRoot)
		return nil, fmt.Errorf("archive contains no valid problems (no manifest.json found)")
	}

	return results, nil
}

func validateProblemDir(ctx context.Context, dir string, exec executor.Executor) (*ValidatedProblem, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("manifest.json missing or unreadable: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest.json: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}

	stmtBytes, err := os.ReadFile(filepath.Join(dir, "statement.md"))
	if err != nil {
		return nil, fmt.Errorf("statement.md missing: %w", err)
	}

	refDir := filepath.Join(dir, "reference")
	refCode, refLang, err := loadReference(refDir)
	if err != nil {
		return nil, err
	}

	testCases, err := loadAndValidateTestCases(filepath.Join(dir, "tests"))
	if err != nil {
		return nil, err
	}

	if err := runReferenceTests(ctx, exec, manifest, refCode, refLang, testCases); err != nil {
		return nil, err
	}

	return &ValidatedProblem{
		Manifest:      manifest,
		Statement:     string(stmtBytes),
		ReferenceCode: refCode,
		ReferenceLang: refLang,
		TestCases:     testCases,
		Dir:           dir,
	}, nil
}

func validateManifest(m Manifest) error {
	if strings.TrimSpace(m.Title) == "" {
		return fmt.Errorf("manifest.json: title is required")
	}
	if m.TimeLimitMs <= 0 || m.TimeLimitMs > 30000 {
		return fmt.Errorf("manifest.json: time_limit_ms must be between 1 and 30000")
	}
	if m.MemoryLimitMb <= 0 || m.MemoryLimitMb > 1024 {
		return fmt.Errorf("manifest.json: memory_limit_mb must be between 1 and 1024")
	}
	return nil
}

var extToLang = map[string]string{
	".py":   "python",
	".go":   "go",
	".cpp":  "cpp",
	".java": "java",
}

func loadReference(refDir string) (code, lang string, err error) {
	if _, err := os.Stat(refDir); os.IsNotExist(err) {
		return "", "", fmt.Errorf("reference/ directory is missing")
	}
	entries, err := os.ReadDir(refDir)
	if err != nil {
		return "", "", fmt.Errorf("reading reference/: %w", err)
	}

	var found []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if _, ok := extToLang[ext]; ok {
			found = append(found, e.Name())
		}
	}
	if len(found) == 0 {
		return "", "", fmt.Errorf("reference/: no solution file found (.py, .go, .cpp, .java)")
	}
	if len(found) > 1 {
		return "", "", fmt.Errorf("reference/: multiple solution files found, expected exactly one")
	}

	ext := strings.ToLower(filepath.Ext(found[0]))
	lang = extToLang[ext]
	codeBytes, err := os.ReadFile(filepath.Join(refDir, found[0]))
	if err != nil {
		return "", "", fmt.Errorf("reading reference solution: %w", err)
	}
	return string(codeBytes), lang, nil
}

func loadAndValidateTestCases(testsDir string) ([]TestCase, error) {
	if _, err := os.Stat(testsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("tests/ directory is missing")
	}

	cases, err := loadTestCases(testsDir)
	if err != nil {
		return nil, fmt.Errorf("loading test cases: %w", err)
	}
	if len(cases) == 0 {
		return nil, fmt.Errorf("tests/: at least one test case (.in/.out pair) is required")
	}
	if len(cases) > MaxTestCases {
		return nil, fmt.Errorf("tests/: too many test cases (%d, max %d)", len(cases), MaxTestCases)
	}
	return cases, nil
}

func runReferenceTests(ctx context.Context, exec executor.Executor, manifest Manifest, code, lang string, tests []TestCase) error {
	timeLimit := time.Duration(manifest.TimeLimitMs) * time.Millisecond
	memLimit := int64(manifest.MemoryLimitMb) * 1024 * 1024

	for _, tc := range tests {
		result, err := exec.Run(ctx, executor.ExecutionRequest{
			Code:        code,
			Language:    executor.Language(lang),
			Stdin:       tc.Input,
			TimeLimit:   timeLimit,
			MemoryLimit: memLimit,
		})
		if err != nil {
			return fmt.Errorf("test %q: executor error: %w", tc.Name, err)
		}
		if !Match(result.Stdout, tc.Expected) {
			return fmt.Errorf("test %q: reference solution output %q, expected %q", tc.Name, NormalizeOutput(result.Stdout), NormalizeOutput(tc.Expected))
		}
	}
	return nil
}

func extractTarGz(r io.Reader, destDir string, maxBytes int64) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("invalid gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var totalBytes int64

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		target, err := safeTarget(destDir, hdr.Name)
		if err != nil {
			return err
		}

		n, err := extractEntry(hdr, tr, target, maxBytes-totalBytes)
		if err != nil {
			return err
		}
		totalBytes += n
		if totalBytes > maxBytes {
			return fmt.Errorf("extracted archive exceeds %d MB", maxBytes/(1024*1024))
		}
	}
	return nil
}

func safeTarget(destDir, name string) (string, error) {
	cleanName := filepath.Clean(name)
	if strings.HasPrefix(cleanName, "..") {
		return "", fmt.Errorf("path traversal attempt: %q", name)
	}
	target := filepath.Join(destDir, cleanName)
	if !strings.HasPrefix(target, destDir+string(os.PathSeparator)) && target != destDir {
		return "", fmt.Errorf("path traversal attempt: %q", name)
	}
	return target, nil
}

func extractEntry(hdr *tar.Header, tr *tar.Reader, target string, remaining int64) (int64, error) {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return 0, os.MkdirAll(target, 0o755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return 0, err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return 0, err
		}
		n, copyErr := io.Copy(f, io.LimitReader(tr, remaining+1))
		if closeErr := f.Close(); closeErr != nil && copyErr == nil {
			return n, closeErr
		}
		return n, copyErr
	}
	return 0, nil
}

func extractArchive(r io.ReadSeeker, size int64, destDir string) error {
	var magic [2]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking archive: %w", err)
	}
	switch {
	case magic[0] == 0x1f && magic[1] == 0x8b:
		return extractTarGz(r, destDir, MaxExtractedBytes)
	case magic[0] == 'P' && magic[1] == 'K':
		ra, ok := r.(io.ReaderAt)
		if !ok {
			return fmt.Errorf("zip extraction requires a seekable reader")
		}
		return extractZip(ra, size, destDir, MaxExtractedBytes)
	default:
		return fmt.Errorf("unsupported archive format (expected .tar.gz or .zip)")
	}
}

func extractZip(r io.ReaderAt, size int64, destDir string, maxBytes int64) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("invalid zip: %w", err)
	}

	var totalBytes int64
	for _, f := range zr.File {
		if f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		target, err := safeTarget(destDir, f.Name)
		if err != nil {
			return err
		}
		n, err := extractZipEntry(f, target, maxBytes-totalBytes)
		if err != nil {
			return err
		}
		totalBytes += n
		if totalBytes > maxBytes {
			return fmt.Errorf("extracted archive exceeds %d MB", maxBytes/(1024*1024))
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, target string, remaining int64) (int64, error) {
	if f.FileInfo().IsDir() {
		return 0, os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, err
	}
	rc, err := f.Open()
	if err != nil {
		return 0, fmt.Errorf("opening zip entry %q: %w", f.Name, err)
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		_ = rc.Close()
		return 0, err
	}
	n, copyErr := io.Copy(out, io.LimitReader(rc, remaining+1))
	closeErr := out.Close()
	_ = rc.Close()
	if copyErr != nil {
		return n, copyErr
	}
	return n, closeErr
}
