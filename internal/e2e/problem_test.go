package e2e_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type problemResp struct {
	Problem struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		Difficulty    string `json:"difficulty"`
		TimeLimitMs   int    `json:"time_limit_ms"`
		MemoryLimitMb int    `json:"memory_limit_mb"`
		TestCount     *int   `json:"test_count"`
	} `json:"problem"`
}

type problemsListResp struct {
	Problems []struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		Difficulty string `json:"difficulty"`
	} `json:"problems"`
}

func TestProblem_List(t *testing.T) {
	resp := do(t, http.MethodGet, "/api/problems", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list problemsListResp
	decodeJSON(t, resp, &list)
	assert.NotEmpty(t, list.Problems)

	var found bool
	for _, p := range list.Problems {
		if p.ID == "test-problem" {
			found = true
			assert.Equal(t, "Test Add", p.Title)
			assert.Equal(t, "easy", p.Difficulty)
		}
	}
	assert.True(t, found, "test-problem not found in list")
}

func TestProblem_GetByID(t *testing.T) {
	resp := do(t, http.MethodGet, "/api/problems/test-problem", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var p problemResp
	decodeJSON(t, resp, &p)
	assert.Equal(t, "test-problem", p.Problem.ID)
	assert.Equal(t, "Test Add", p.Problem.Title)
	assert.Equal(t, "easy", p.Problem.Difficulty)
	assert.Equal(t, 1000, p.Problem.TimeLimitMs)
	assert.Equal(t, 256, p.Problem.MemoryLimitMb)
	require.NotNil(t, p.Problem.TestCount)
	assert.Equal(t, 2, *p.Problem.TestCount)
}

func TestProblem_GetByID_NotFound(t *testing.T) {
	resp := do(t, http.MethodGet, "/api/problems/nonexistent-problem", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "PROBLEM_NOT_FOUND", errCode(t, resp))
}

func problemArchiveTarGz(t *testing.T, title string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range problemFiles(title) {
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
	return buf.Bytes()
}

func problemArchiveZip(t *testing.T, title string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range problemFiles(title) {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func problemFiles(title string) map[string]string {
	return map[string]string{
		"manifest.json":         `{"title":"` + title + `","time_limit_ms":1000,"memory_limit_mb":256}`,
		"statement.md":          "# " + title + "\n",
		"reference/solution.py": "print(3)\n",
		"tests/01.in":           "x\n",
		"tests/01.out":          "3\n",
	}
}

func doUpload(t *testing.T, srv *httptest.Server, archiveData []byte, filename, visibility, token string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(archiveData)
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("visibility", visibility))
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/problems", &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	return resp
}

func doUploadVersion(t *testing.T, srv *httptest.Server, slug string, archiveData []byte, token string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "problem.tar.gz")
	require.NoError(t, err)
	_, err = part.Write(archiveData)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/problems/"+slug+"/versions", &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	return resp
}

func TestProblem_Upload_TarGz(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveTarGz(t, "Upload TarGz Test"), "problem.tar.gz", "public", token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var result map[string]any
	decodeJSON(t, resp, &result)
	slug, _ := result["slug"].(string)
	require.NotEmpty(t, slug)
	assert.Equal(t, float64(1), result["version"])

	resp2 := doOnServer(t, srv, http.MethodGet, "/api/problems/"+slug, nil, "")
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	resp2.Body.Close()
}

func TestProblem_Upload_Zip(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveZip(t, "Upload Zip Test"), "problem.zip", "public", token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var result map[string]any
	decodeJSON(t, resp, &result)
	slug, _ := result["slug"].(string)
	require.NotEmpty(t, slug)
}

func TestProblem_Upload_Unauthorized(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveTarGz(t, "Unauthorized Test"), "problem.tar.gz", "public", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestProblem_Upload_InvalidVisibility(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveTarGz(t, "Bad Visibility Test"), "problem.tar.gz", "invalid", token1)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestProblem_Upload_InvalidFormat(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, []byte("not an archive"), "problem.bin", "public", token1)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestProblem_Mine(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveTarGz(t, "Mine Private Test"), "mine.tar.gz", "private", token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var uploadResult map[string]any
	decodeJSON(t, resp, &uploadResult)
	slug, _ := uploadResult["slug"].(string)
	require.NotEmpty(t, slug)

	resp2 := doOnServer(t, srv, http.MethodGet, "/api/problems/mine", nil, token1)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var mine struct {
		Problems []struct {
			Id         string `json:"id"`
			Visibility string `json:"visibility"`
		} `json:"problems"`
	}
	decodeJSON(t, resp2, &mine)
	var found bool
	for _, p := range mine.Problems {
		if p.Id == slug {
			found = true
			assert.Equal(t, "private", p.Visibility)
		}
	}
	assert.True(t, found, "uploaded problem not found in /mine")
}

func TestProblem_Upload_PrivateNotAccessibleByOther(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveTarGz(t, "Private Access Test"), "private.tar.gz", "private", token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var uploadResult map[string]any
	decodeJSON(t, resp, &uploadResult)
	slug, _ := uploadResult["slug"].(string)
	require.NotEmpty(t, slug)

	resp2 := doOnServer(t, srv, http.MethodGet, "/api/problems/"+slug, nil, token2)
	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
	resp2.Body.Close()
}

func TestProblem_Upload_NewVersion(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveTarGz(t, "Version Test"), "v1.tar.gz", "public", token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var v1Result map[string]any
	decodeJSON(t, resp, &v1Result)
	slug, _ := v1Result["slug"].(string)
	require.NotEmpty(t, slug)

	resp2 := doUploadVersion(t, srv, slug, problemArchiveTarGz(t, "Version Test"), token1)
	require.Equal(t, http.StatusCreated, resp2.StatusCode)
	var v2Result map[string]any
	decodeJSON(t, resp2, &v2Result)
	assert.Equal(t, slug, v2Result["slug"])
	assert.Equal(t, float64(2), v2Result["version"])
}

func TestProblem_Upload_NewVersion_NotOwner(t *testing.T) {
	srv := newUploadServer(t)
	resp := doUpload(t, srv, problemArchiveTarGz(t, "Not Owner Version Test"), "v1.tar.gz", "public", token1)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var v1Result map[string]any
	decodeJSON(t, resp, &v1Result)
	slug, _ := v1Result["slug"].(string)
	require.NotEmpty(t, slug)

	resp2 := doUploadVersion(t, srv, slug, problemArchiveTarGz(t, "Not Owner Version Test"), token2)
	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)
	resp2.Body.Close()
}
