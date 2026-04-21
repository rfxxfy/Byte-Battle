package problems

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	sqlcdb "bytebattle/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedBuiltins(ctx context.Context, pool *pgxpool.Pool, store *Store) error {
	slugEntries, err := os.ReadDir(store.baseDir)
	if err != nil {
		return fmt.Errorf("reading problems dir: %w", err)
	}

	for _, slugEntry := range slugEntries {
		if !slugEntry.IsDir() {
			continue
		}
		if err := seedSlug(ctx, pool, store, slugEntry.Name()); err != nil {
			return fmt.Errorf("seeding %q: %w", slugEntry.Name(), err)
		}
	}
	return nil
}

func seedSlug(ctx context.Context, pool *pgxpool.Pool, store *Store, slug string) error {
	q := sqlcdb.New(pool)

	catalog, lookupErr := q.GetProblemCatalogBySlug(ctx, slug)
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}
	if lookupErr == nil && catalog.CurrentVersionID.Valid {
		return nil // already fully seeded
	}
	catalogExists := lookupErr == nil

	latestVersion, err := latestVersionDir(filepath.Join(store.baseDir, slug))
	if err != nil {
		return err
	}

	artifactPath := slug + "/" + latestVersion
	problem, err := store.GetByPath(artifactPath)
	if err != nil {
		return fmt.Errorf("loading problem: %w", err)
	}

	sha, err := dirSHA256(filepath.Join(store.baseDir, slug, latestVersion))
	if err != nil {
		return fmt.Errorf("computing sha256: %w", err)
	}

	refLang, err := detectReferenceLanguage(filepath.Join(store.baseDir, slug, latestVersion, "reference"))
	if err != nil {
		return fmt.Errorf("detecting reference language: %w", err)
	}

	if err := insertProblemTx(ctx, pool, insertProblemArgs{
		slug:          slug,
		catalogID:     catalog.ID,
		catalogExists: catalogExists,
		problem:       problem,
		artifactPath:  artifactPath,
		sha:           sha,
		refLang:       refLang,
		version:       versionStrToInt(latestVersion),
	}); err != nil {
		return err
	}

	log.Printf("seeded built-in problem %q (artifact=%s)", slug, artifactPath)
	return nil
}

type insertProblemArgs struct {
	slug          string
	catalogID     int64
	catalogExists bool
	problem       *Problem
	artifactPath  string
	sha           string
	refLang       string
	version       int
}

func insertProblemTx(ctx context.Context, pool *pgxpool.Pool, args insertProblemArgs) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := sqlcdb.New(tx)

	catalogID := args.catalogID
	if !args.catalogExists {
		newCatalog, err := qtx.CreateProblemCatalog(ctx, sqlcdb.CreateProblemCatalogParams{
			Slug:        args.slug,
			OwnerUserID: uuid.NullUUID{Valid: false},
			Visibility:  "public",
			Status:      "published",
			Title:       args.problem.Manifest.Title,
		})
		if err != nil {
			return fmt.Errorf("create problem catalog: %w", err)
		}
		catalogID = newCatalog.ID
	}

	pv, err := qtx.CreateProblemVersion(ctx, sqlcdb.CreateProblemVersionParams{
		ProblemID:         catalogID,
		Version:           int32(args.version),
		ArtifactPath:      args.artifactPath,
		ArtifactSha256:    args.sha,
		LimitsTimeMs:      int32(args.problem.Manifest.TimeLimitMs),
		LimitsMemoryKb:    int32(args.problem.Manifest.MemoryLimitMb * 1024),
		CheckerType:       "diff",
		ReferenceLanguage: args.refLang,
		CreatedByUserID:   uuid.NullUUID{Valid: false},
	})
	if err != nil {
		return fmt.Errorf("create problem version: %w", err)
	}

	if err := qtx.SetProblemCurrentVersion(ctx, sqlcdb.SetProblemCurrentVersionParams{
		ID:               catalogID,
		CurrentVersionID: pgtype.Int8{Int64: pv.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set current version: %w", err)
	}

	return tx.Commit(ctx)
}

func latestVersionDir(slugDir string) (string, error) {
	entries, err := os.ReadDir(slugDir)
	if err != nil {
		return "", fmt.Errorf("reading slug dir: %w", err)
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "v") {
			versions = append(versions, e.Name())
		}
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no version directories found in %q", slugDir)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versionStrToInt(versions[i]) < versionStrToInt(versions[j])
	})
	return versions[len(versions)-1], nil
}

func dirSHA256(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		h.Write([]byte(filepath.ToSlash(rel) + "\n"))
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func detectReferenceLanguage(refDir string) (string, error) {
	entries, err := os.ReadDir(refDir)
	if err != nil {
		return "", fmt.Errorf("reading reference dir: %w", err)
	}
	extToLang := map[string]string{
		".py":   "python",
		".go":   "go",
		".cpp":  "cpp",
		".java": "java",
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if lang, ok := extToLang[ext]; ok {
			return lang, nil
		}
	}
	return "", fmt.Errorf("no supported reference solution found in %q", refDir)
}

func versionStrToInt(v string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(v, "v"))
	if err != nil || n <= 0 {
		return 1
	}
	return n
}
