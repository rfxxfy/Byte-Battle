package e2e_test

import (
	"net/http"
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
