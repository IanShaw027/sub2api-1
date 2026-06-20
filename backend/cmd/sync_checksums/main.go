package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
)

type migrationChecksum struct {
	Filename string
	Checksum string
}

func main() {
	var backupFile string
	var restoreFile string
	flag.StringVar(&backupFile, "backup-file", "", "write current schema_migrations checksums to this JSON file before syncing")
	flag.StringVar(&restoreFile, "restore-file", "", "restore schema_migrations checksums from this JSON file and exit")
	flag.Parse()

	dsn := strings.TrimSpace(os.Getenv("SUB2API_DATABASE_DSN"))
	if dsn == "" && flag.NArg() >= 1 {
		dsn = flag.Arg(0)
	}
	if dsn == "" {
		fmt.Println("Usage: SUB2API_DATABASE_DSN='<database_dsn>' sync_checksums [--backup-file path] [--restore-file path]")
		fmt.Println("Legacy argv input is still accepted for manual use, but deploy scripts should prefer the environment variable to keep secrets out of process arguments.")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	if strings.TrimSpace(restoreFile) != "" {
		items, err := readChecksumSnapshot(restoreFile)
		if err != nil {
			log.Fatalf("read checksum snapshot: %v", err)
		}
		restored, err := restoreDatabaseChecksums(ctx, db, items)
		if err != nil {
			log.Fatalf("restore checksum snapshot: %v", err)
		}
		fmt.Printf("checksum restore complete: restored=%d checked=%d\n", restored, len(items))
		return
	}

	fileChecksums, err := loadMigrationChecksums(migrations.FS)
	if err != nil {
		log.Fatalf("load migration checksums: %v", err)
	}

	dbChecksums, err := loadDatabaseChecksums(ctx, db)
	if err != nil {
		log.Fatalf("load database checksums: %v", err)
	}

	if strings.TrimSpace(backupFile) != "" {
		if err := writeChecksumSnapshot(backupFile, dbChecksums); err != nil {
			log.Fatalf("write checksum snapshot: %v", err)
		}
		fmt.Printf("checksum snapshot written: %s\n", backupFile)
	}

	updated, err := syncDatabaseChecksums(ctx, db, fileChecksums, dbChecksums)
	if err != nil {
		log.Fatalf("sync checksum: %v", err)
	}

	fmt.Printf("checksum sync complete: updated=%d checked=%d\n", updated, len(fileChecksums))
}

func loadMigrationChecksums(fsys fs.FS) ([]migrationChecksum, error) {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	items := make([]migrationChecksum, 0, len(files))
	for _, name := range files {
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			continue
		}
		sum := sha256.Sum256([]byte(trimmed))
		items = append(items, migrationChecksum{
			Filename: name,
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	return items, nil
}

func writeChecksumSnapshot(path string, items map[string]string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	snapshot := make([]migrationChecksum, 0, len(items))
	for filename, checksum := range items {
		snapshot = append(snapshot, migrationChecksum{Filename: filename, Checksum: checksum})
	}
	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].Filename < snapshot[j].Filename
	})
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func readChecksumSnapshot(path string) ([]migrationChecksum, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	var items []migrationChecksum
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.Filename) == "" {
			return nil, fmt.Errorf("snapshot contains empty filename")
		}
	}
	return items, nil
}

func restoreDatabaseChecksums(ctx context.Context, db *sql.DB, items []migrationChecksum) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	var restored int
	for _, item := range items {
		result, err := tx.ExecContext(ctx,
			"UPDATE schema_migrations SET checksum = $1 WHERE filename = $2",
			item.Checksum,
			item.Filename,
		)
		if err != nil {
			_ = tx.Rollback()
			return restored, fmt.Errorf("update checksum for %s: %w", item.Filename, err)
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			restored++
		}
	}
	if err := tx.Commit(); err != nil {
		return restored, err
	}
	return restored, nil
}

func syncDatabaseChecksums(ctx context.Context, db *sql.DB, fileChecksums []migrationChecksum, dbChecksums map[string]string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	var updated int
	for _, item := range fileChecksums {
		dbChecksum, ok := dbChecksums[item.Filename]
		if !ok || dbChecksum == item.Checksum {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE schema_migrations SET checksum = $1 WHERE filename = $2",
			item.Checksum,
			item.Filename,
		); err != nil {
			_ = tx.Rollback()
			return updated, fmt.Errorf("update checksum for %s: %w", item.Filename, err)
		}
		updated++
		fmt.Printf("updated %s\n", item.Filename)
	}
	if err := tx.Commit(); err != nil {
		return updated, err
	}
	return updated, nil
}

func loadDatabaseChecksums(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT filename, checksum FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make(map[string]string)
	for rows.Next() {
		var filename string
		var checksum string
		if err := rows.Scan(&filename, &checksum); err != nil {
			return nil, err
		}
		items[filename] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
