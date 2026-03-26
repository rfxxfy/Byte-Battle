package problems

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"bytebattle/internal/apierr"
	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UploadResult struct {
	Slug    string
	Title   string
	Version int
}

func UploadProblem(ctx context.Context, pool *pgxpool.Pool, store *Store, validated *ValidatedProblem, ownerID uuid.UUID, visibility string) (*UploadResult, error) {
	if validated.Manifest.Slug != "" {
		return uploadNewVersion(ctx, pool, store, validated, validated.Manifest.Slug, ownerID)
	}

	q := sqlcdb.New(pool)

	slug, err := generateSlug(ctx, q, validated.Manifest.Title)
	if err != nil {
		return nil, err
	}

	sha, err := dirSHA256(validated.Dir)
	if err != nil {
		return nil, fmt.Errorf("computing sha256: %w", err)
	}

	artifactPath := slug + "/v1"

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := sqlcdb.New(tx)

	catalog, err := qtx.CreateProblemCatalog(ctx, sqlcdb.CreateProblemCatalogParams{
		Slug:        slug,
		OwnerUserID: uuid.NullUUID{UUID: ownerID, Valid: true},
		Visibility:  visibility,
		Status:      "published",
		Title:       validated.Manifest.Title,
	})
	if err != nil {
		return nil, fmt.Errorf("create problem catalog: %w", err)
	}

	pv, err := qtx.CreateProblemVersion(ctx, sqlcdb.CreateProblemVersionParams{
		ProblemID:         catalog.ID,
		Version:           1,
		ArtifactPath:      artifactPath,
		ArtifactSha256:    sha,
		LimitsTimeMs:      int32(validated.Manifest.TimeLimitMs),
		LimitsMemoryKb:    int32(validated.Manifest.MemoryLimitMb * 1024),
		CheckerType:       "diff",
		ReferenceLanguage: validated.ReferenceLang,
		CreatedByUserID:   uuid.NullUUID{UUID: ownerID, Valid: true},
		TestCaseCount:     int32(len(validated.TestCases)),
		Difficulty:        validated.Manifest.Difficulty,
	})
	if err != nil {
		return nil, fmt.Errorf("create problem version: %w", err)
	}

	if err := qtx.SetProblemCurrentVersion(ctx, sqlcdb.SetProblemCurrentVersionParams{
		ID:               catalog.ID,
		CurrentVersionID: pgtype.Int8{Int64: pv.ID, Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("set current version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if err := moveProblemDir(validated.Dir, store.baseDir, artifactPath); err != nil {
		log.Printf("CRITICAL: committed problem %q to DB but file move failed (artifact=%s): %v — manual recovery required", slug, artifactPath, err)
		return nil, fmt.Errorf("moving problem files: %w", err)
	}

	store.mu.Lock()
	delete(store.cache, artifactPath)
	store.mu.Unlock()

	return &UploadResult{Slug: slug, Title: validated.Manifest.Title, Version: 1}, nil
}

func UploadNewVersion(ctx context.Context, pool *pgxpool.Pool, store *Store, validated *ValidatedProblem, slug string, ownerID uuid.UUID) (*UploadResult, error) {
	return uploadNewVersion(ctx, pool, store, validated, slug, ownerID)
}

func uploadNewVersion(ctx context.Context, pool *pgxpool.Pool, store *Store, validated *ValidatedProblem, slug string, ownerID uuid.UUID) (*UploadResult, error) {
	q := sqlcdb.New(pool)

	catalog, err := q.GetProblemCatalogBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierr.New(apierr.ErrProblemNotFound, "problem not found")
	}
	if err != nil {
		return nil, err
	}

	sha, err := dirSHA256(validated.Dir)
	if err != nil {
		return nil, fmt.Errorf("computing sha256: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := sqlcdb.New(tx)

	if _, err := qtx.LockProblemForUpdate(ctx, catalog.ID); err != nil {
		return nil, fmt.Errorf("locking problem row: %w", err)
	}

	maxVer, err := qtx.GetMaxProblemVersion(ctx, catalog.ID)
	if err != nil {
		return nil, fmt.Errorf("get max version: %w", err)
	}
	newVersion := int(maxVer) + 1
	artifactPath := fmt.Sprintf("%s/v%d", slug, newVersion)

	pv, err := qtx.CreateProblemVersion(ctx, sqlcdb.CreateProblemVersionParams{
		ProblemID:         catalog.ID,
		Version:           int32(newVersion),
		ArtifactPath:      artifactPath,
		ArtifactSha256:    sha,
		LimitsTimeMs:      int32(validated.Manifest.TimeLimitMs),
		LimitsMemoryKb:    int32(validated.Manifest.MemoryLimitMb * 1024),
		CheckerType:       "diff",
		ReferenceLanguage: validated.ReferenceLang,
		CreatedByUserID:   uuid.NullUUID{UUID: ownerID, Valid: true},
		TestCaseCount:     int32(len(validated.TestCases)),
		Difficulty:        validated.Manifest.Difficulty,
	})
	if err != nil {
		return nil, fmt.Errorf("create problem version: %w", err)
	}

	if err := qtx.SetProblemCurrentVersion(ctx, sqlcdb.SetProblemCurrentVersionParams{
		ID:               catalog.ID,
		CurrentVersionID: pgtype.Int8{Int64: pv.ID, Valid: true},
	}); err != nil {
		return nil, fmt.Errorf("set current version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if err := moveProblemDir(validated.Dir, store.baseDir, artifactPath); err != nil {
		log.Printf("CRITICAL: committed version %d for problem %q to DB but file move failed (artifact=%s): %v — manual recovery required", newVersion, slug, artifactPath, err)
		return nil, fmt.Errorf("moving problem files: %w", err)
	}

	store.mu.Lock()
	delete(store.cache, artifactPath)
	store.mu.Unlock()

	return &UploadResult{Slug: slug, Title: validated.Manifest.Title, Version: newVersion}, nil
}

var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

func titleToKebab(title string) string {
	lower := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return '-'
	}, title)
	cleaned := nonAlphanumRE.ReplaceAllString(lower, "-")
	cleaned = strings.Trim(cleaned, "-")
	if len(cleaned) > 40 {
		cleaned = cleaned[:40]
		if idx := strings.LastIndex(cleaned, "-"); idx > 0 {
			cleaned = cleaned[:idx]
		}
	}
	return cleaned
}

func generateSlug(ctx context.Context, q *sqlcdb.Queries, title string) (string, error) {
	base := titleToKebab(title)
	if base == "" {
		base = "problem"
	}
	for range 10 {
		suffix, err := randomHex(2) // 4 hex chars
		if err != nil {
			return "", err
		}
		slug := base + "-" + suffix
		_, err = q.GetProblemCatalogBySlug(ctx, slug)
		if errors.Is(err, pgx.ErrNoRows) {
			return slug, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("failed to generate unique slug after 10 attempts")
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func moveProblemDir(srcDir, baseDir, artifactPath string) error {
	dest := filepath.Join(baseDir, filepath.FromSlash(artifactPath))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.Rename(srcDir, dest)
}
