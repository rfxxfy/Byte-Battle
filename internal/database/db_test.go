package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"net"

	_ "github.com/lib/pq"

	"github.com/aarondl/sqlboiler/v4/boil"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/stretchr/testify/suite"

	"bytebattle/internal/config"
	"bytebattle/internal/database/models"
)

type DBSuite struct {
	suite.Suite
	pg     *embeddedpostgres.EmbeddedPostgres
	client *Client
	ctx    context.Context
}

// ---------- utils ----------

func freePort(tb testing.TB) int {
	tb.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to get free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func projectRootFromThisFile() string {
	_, file, _, _ := runtime.Caller(0)
	// internal/database/db_test.go -> корень проекта
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func uniqueSuffix() string {
	return fmt.Sprintf("_%d", time.Now().UnixNano())
}


func isUpMigration(name string) bool {
	// только up файлы и обычные *.sql с секцией -- +goose Up
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".sql") {
	return false
	}
	// исключаем down/rollback/undo
	if strings.HasSuffix(lower, ".down.sql") ||
	strings.HasSuffix(lower, ".rollback.sql") ||
	strings.HasSuffix(lower, ".undo.sql") {
	return false
	}
	// поддерживаем два формата:
	// - 001_name.up.sql (приоритет)
	// - 001_name.sql (с секцией -- +goose Up внутри)
	return true
	}

// читает Up-секции из всех .sql в schema/ и применяет их в лексикографическом порядке
func applyMigrations(db *sql.DB, dir string) error {
	var files []string
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isUpMigration(d.Name()) {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walk dir %s: %w", dir, err)
	}

	sort.Strings(files)

	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		upSQL := extractGooseUp(string(b))
		if strings.TrimSpace(upSQL) == "" {
			continue
		}
		if _, err := db.Exec(upSQL); err != nil {
			return fmt.Errorf("apply %s: %w", path, err)
		}
	}
	return nil
}
// берет из файла содержимое после строки -- +goose Up до -- +goose Down
func extractGooseUp(content string) string {
	lines := strings.Split(content, "\n")
	inUp := false
	var b strings.Builder
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "-- +goose Up") {
			inUp = true
			continue
		}
		if strings.HasPrefix(trim, "-- +goose Down") {
			break
		}
		if inUp {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	// если маркера не было то считаем, что весь файл это Up (на случай простых миграций)
	if !inUp {
		return content
	}
	return b.String()
}

// ---------- setup/teardown ----------

func (s *DBSuite) SetupSuite() {
	s.ctx = context.Background()

	port := freePort(s.T())
dataDir := filepath.Join(os.TempDir(), fmt.Sprintf("bb-pg-%d", time.Now().UnixNano()))
s.T().Logf("embedded PG data dir: %s, port: %d", dataDir, port)

// поднимаем embedded PG с явным user/password и уникальным data dir
cfg := embeddedpostgres.DefaultConfig().
    DataPath(dataDir).
    Port(uint32(port)).
    Database("bytebattle_test").
    Username("bytebattle").
    Password("bytebattle")

s.pg = embeddedpostgres.NewDatabase(cfg)
s.Require().NoError(s.pg.Start(), "failed to start embedded Postgres")

// коннектимся для миграций по URL-DSN (пароль точно передастся)
dsn := fmt.Sprintf("postgres://bytebattle:bytebattle@127.0.0.1:%d/bytebattle_test?sslmode=disable", port)
s.T().Log("migrations DSN:", dsn) // пароль видно, но это только в локальном логе тестов
rawDB, err := sql.Open("postgres", dsn)
s.Require().NoError(err)
s.Require().NoError(rawDB.Ping())

migrationsDir := filepath.Join(projectRootFromThisFile(), "schema")
s.Require().NoError(applyMigrations(rawDB, migrationsDir), "migrations failed")
_ = rawDB.Close()

// создаем клиент с теми же кредами
cfgApp := &config.DatabaseConfig{
    Host:     "127.0.0.1",
    Port:     port,
    User:     "bytebattle",
    Password: "bytebattle",
    Name:     "bytebattle_test",
    SSLMode:  "disable",
}
client, err := NewClient(cfgApp)
s.Require().NoError(err)
s.client = client

boil.SetDB(client.DB)
}

func (s *DBSuite) TearDownSuite() {
	if s.client != nil {
		_ = s.client.Close()
	}
	if s.pg != nil {
		_ = s.pg.Stop()
	}
}

// ---------- tests (users, problems) ----------

func (s *DBSuite) Test_CreateUser_And_GetUserByUsername() {
	ctx := s.ctx
	sfx := uniqueSuffix()

	u, err := s.client.CreateUser(ctx, "tester"+sfx, "tester"+sfx+"@example.com", "hash")
	s.Require().NoError(err)
	s.Require().NotNil(u)
	s.NotZero(u.ID)
	s.Equal("tester"+sfx, u.Username)

	got, err := s.client.GetUserByUsername(ctx, "tester"+sfx)
	s.Require().NoError(err)
	s.Equal(u.ID, got.ID)
	s.Equal("tester"+sfx, got.Username)
	s.Equal("tester"+sfx+"@example.com", got.Email)
}

func (s *DBSuite) Test_GetUserByUsername_NotFound() {
	_, err := s.client.GetUserByUsername(s.ctx, "no_such_user_"+uniqueSuffix())
	s.Require().Error(err)
	s.Contains(err.Error(), "не удалось получить пользователя по имени")
}

func (s *DBSuite) Test_CreateProblem_And_GetProblems() {
	ctx := s.ctx
	sfx := uniqueSuffix()

	p1, err := s.client.CreateProblem(ctx, "Two Sum"+sfx, "desc", "easy", 2, 256)
	s.Require().NoError(err)
	s.NotZero(p1.ID)

	p2, err := s.client.CreateProblem(ctx, "Binary Tree"+sfx, "desc", "medium", 2, 256)
	s.Require().NoError(err)
	s.NotZero(p2.ID)

	all, err := s.client.GetProblems(ctx)
	s.Require().NoError(err)

	found1, found2 := false, false
	for _, pr := range all {
		if pr.ID == p1.ID {
			found1 = true
		}
		if pr.ID == p2.ID {
			found2 = true
		}
	}
	s.True(found1 && found2, "created problems should be in the list")
}

// ---------- tests (duels, solutions) ----------

func (s *DBSuite) Test_CreateDuel_And_UpdateDuelStatus() {
	ctx := s.ctx
	sfx := uniqueSuffix()

	u1, err := s.client.CreateUser(ctx, "p1"+sfx, "p1"+sfx+"@example.com", "hash")
	s.Require().NoError(err)
	u2, err := s.client.CreateUser(ctx, "p2"+sfx, "p2"+sfx+"@example.com", "hash")
	s.Require().NoError(err)
	prob, err := s.client.CreateProblem(ctx, "Quick Task"+sfx, "desc", "easy", 1, 128)
	s.Require().NoError(err)

	duel, err := s.client.CreateDuel(ctx, u1.ID, u2.ID, prob.ID)
	s.Require().NoError(err)
	s.NotZero(duel.ID)
	s.Equal("pending", duel.Status)

	// в схеме допустимые статусы: pending, active, completed, cancelled
	err = s.client.UpdateDuelStatus(ctx, duel.ID, "active")
	s.Require().NoError(err)

	dFromDB, err := models.FindDuel(ctx, s.client.DB, duel.ID)
	s.Require().NoError(err)
	s.Equal("active", dFromDB.Status)
}

func (s *DBSuite) Test_CreateSolution_And_UpdateSolutionStatus() {
	ctx := s.ctx
	sfx := uniqueSuffix()

	u, err := s.client.CreateUser(ctx, "sol_user"+sfx, "sol_user"+sfx+"@example.com", "hash")
	s.Require().NoError(err)
	p, err := s.client.CreateProblem(ctx, "SolProb"+sfx, "desc", "easy", 1, 64)
	s.Require().NoError(err)

	sol, err := s.client.CreateSolution(ctx, u.ID, p.ID, "print('hi')", "python")
	s.Require().NoError(err)
	s.NotZero(sol.ID)
	s.Equal("pending", sol.Status)

	// в схеме допустимые статусы: pending, running, passed, failed
	exec := 123
	mem := 2048
	err = s.client.UpdateSolutionStatus(ctx, sol.ID, "passed", &exec, &mem)
	s.Require().NoError(err)

	solFromDB, err := models.FindSolution(ctx, s.client.DB, sol.ID)
	s.Require().NoError(err)
	s.Equal("passed", solFromDB.Status)
	s.True(solFromDB.ExecutionTime.Valid)
	s.Equal(exec, solFromDB.ExecutionTime.Int)
	s.True(solFromDB.MemoryUsed.Valid)
	s.Equal(mem, solFromDB.MemoryUsed.Int)
}

func (s *DBSuite) Test_UpdateSolutionStatus_NotFound() {
	ctx := s.ctx
	err := s.client.UpdateSolutionStatus(ctx, 9999999, "passed", nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "решение не найдено")
}

func TestDBSuite(t *testing.T) {
	suite.Run(t, new(DBSuite))
}