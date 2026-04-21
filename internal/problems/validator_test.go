package problems

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"strings"
	"testing"

	"bytebattle/internal/executor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedOutputExec struct{ out string }

func (e fixedOutputExec) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return executor.ExecutionResult{Stdout: e.out}, nil
}
func (e fixedOutputExec) IsReady() bool { return true }

type notReadyExec struct{}

func (notReadyExec) Run(_ context.Context, _ executor.ExecutionRequest) (executor.ExecutionResult, error) {
	return executor.ExecutionResult{}, nil
}
func (notReadyExec) IsReady() bool { return false }

func buildTarGz(t *testing.T, files map[string]string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return bytes.NewReader(buf.Bytes())
}

func buildZip(t *testing.T, files map[string]string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return bytes.NewReader(buf.Bytes())
}

func validFiles() map[string]string {
	return map[string]string{
		"manifest.json":         `{"title":"Test","time_limit_ms":1000,"memory_limit_mb":256}`,
		"statement.md":          "# Test\n",
		"reference/solution.py": "print(3)\n",
		"tests/01.in":           "x\n",
		"tests/01.out":          "3\n",
	}
}

func TestSafeTarget_PathTraversal(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"dot-dot", "../escape"},
		{"nested", "sub/../../escape"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := safeTarget("/tmp/dest", tc.path)
			require.Error(t, err)
		})
	}
}

func TestSafeTarget_AbsolutePathNeutralized(t *testing.T) {
	target, err := safeTarget("/tmp/dest", "/etc/passwd")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/dest/etc/passwd", target)
}

func TestSafeTarget_OK(t *testing.T) {
	target, err := safeTarget("/tmp/dest", "a/b/c.txt")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/dest/a/b/c.txt", target)
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	r := buildTarGz(t, map[string]string{"../escape.txt": "bad"})
	err := extractTarGz(r, t.TempDir(), MaxExtractedBytes)
	require.ErrorContains(t, err, "path traversal")
}

func TestExtractTarGz_SizeLimit(t *testing.T) {
	r := buildTarGz(t, map[string]string{"big.txt": strings.Repeat("A", 1000)})
	err := extractTarGz(r, t.TempDir(), 500)
	require.ErrorContains(t, err, "exceeds")
}

func TestExtractZip_PathTraversal(t *testing.T) {
	r := buildZip(t, map[string]string{"../escape.txt": "bad"})
	err := extractZip(r, int64(r.Len()), t.TempDir(), MaxExtractedBytes)
	require.ErrorContains(t, err, "path traversal")
}

func TestExtractZip_SizeLimit(t *testing.T) {
	r := buildZip(t, map[string]string{"big.txt": strings.Repeat("A", 1000)})
	err := extractZip(r, int64(r.Len()), t.TempDir(), 500)
	require.ErrorContains(t, err, "exceeds")
}

func TestExtractArchive_TarGz(t *testing.T) {
	r := buildTarGz(t, map[string]string{"hello.txt": "hi"})
	require.NoError(t, extractArchive(r, int64(r.Len()), t.TempDir()))
}

func TestExtractArchive_Zip(t *testing.T) {
	r := buildZip(t, map[string]string{"hello.txt": "hi"})
	require.NoError(t, extractArchive(r, int64(r.Len()), t.TempDir()))
}

func TestExtractArchive_UnknownFormat(t *testing.T) {
	r := bytes.NewReader([]byte("not an archive"))
	err := extractArchive(r, int64(r.Len()), t.TempDir())
	require.ErrorContains(t, err, "unsupported archive format")
}

func TestValidateArchive_ExecutorNotReady(t *testing.T) {
	r := buildTarGz(t, validFiles())
	_, err := ValidateArchive(context.Background(), r, int64(r.Len()), notReadyExec{})
	require.ErrorContains(t, err, "executor not ready")
}

func TestValidateArchive_TooLarge(t *testing.T) {
	r := buildTarGz(t, validFiles())
	_, err := ValidateArchive(context.Background(), r, MaxArchiveBytes+1, fixedOutputExec{"3"})
	require.ErrorContains(t, err, "archive too large")
}

func TestValidateArchive_SingleProblem_TarGz(t *testing.T) {
	r := buildTarGz(t, validFiles())
	vps, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.NoError(t, err)
	require.Len(t, vps, 1)
	t.Cleanup(func() { os.RemoveAll(vps[0].Dir) })
	assert.Equal(t, "Test", vps[0].Manifest.Title)
	assert.Equal(t, "python", vps[0].ReferenceLang)
	assert.Len(t, vps[0].TestCases, 1)
}

func TestValidateArchive_SingleProblem_Zip(t *testing.T) {
	r := buildZip(t, validFiles())
	vps, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.NoError(t, err)
	require.Len(t, vps, 1)
	t.Cleanup(func() { os.RemoveAll(vps[0].Dir) })
	assert.Equal(t, "Test", vps[0].Manifest.Title)
}

func TestValidateArchive_BatchFormat(t *testing.T) {
	files := make(map[string]string)
	for k, v := range validFiles() {
		files["prob-a/"+k] = v
		files["prob-b/"+k] = v
	}
	r := buildTarGz(t, files)
	vps, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.NoError(t, err)
	require.Len(t, vps, 2)
	for _, vp := range vps {
		t.Cleanup(func() { os.RemoveAll(vp.Dir) })
	}
}

func TestValidateArchive_NoManifest(t *testing.T) {
	files := validFiles()
	delete(files, "manifest.json")
	r := buildTarGz(t, files)
	_, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.ErrorContains(t, err, "no manifest.json found")
}

func TestValidateArchive_InvalidManifest_NoTitle(t *testing.T) {
	files := validFiles()
	files["manifest.json"] = `{"title":"","time_limit_ms":1000,"memory_limit_mb":256}`
	r := buildTarGz(t, files)
	_, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.ErrorContains(t, err, "title is required")
}

func TestValidateArchive_MissingStatement(t *testing.T) {
	files := validFiles()
	delete(files, "statement.md")
	r := buildTarGz(t, files)
	_, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.ErrorContains(t, err, "statement.md missing")
}

func TestValidateArchive_NoTestFiles(t *testing.T) {
	files := validFiles()
	delete(files, "tests/01.in")
	delete(files, "tests/01.out")
	r := buildTarGz(t, files)
	_, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.ErrorContains(t, err, "tests/ directory is missing")
}

func TestValidateArchive_ReferenceTestFails(t *testing.T) {
	r := buildTarGz(t, validFiles())
	_, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"wrong"})
	require.ErrorContains(t, err, "reference solution output")
}

func TestValidateArchive_SlugInManifest(t *testing.T) {
	files := validFiles()
	files["manifest.json"] = `{"slug":"existing-abc1","title":"Test","time_limit_ms":1000,"memory_limit_mb":256}`
	r := buildTarGz(t, files)
	vps, err := ValidateArchive(context.Background(), r, int64(r.Len()), fixedOutputExec{"3"})
	require.NoError(t, err)
	require.Len(t, vps, 1)
	t.Cleanup(func() { os.RemoveAll(vps[0].Dir) })
	assert.Equal(t, "existing-abc1", vps[0].Manifest.Slug)
}
