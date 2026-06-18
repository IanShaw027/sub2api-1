package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
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
	dsn := strings.TrimSpace(os.Getenv("SUB2API_DATABASE_DSN"))
	if dsn == "" && len(os.Args) >= 2 {
		dsn = os.Args[1]
	}
	if dsn == "" {
		fmt.Println("Usage: SUB2API_DATABASE_DSN='<database_dsn>' sync_checksums")
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

	fileChecksums, err := loadMigrationChecksums(migrations.FS)
	if err != nil {
		log.Fatalf("load migration checksums: %v", err)
	}

	dbChecksums, err := loadDatabaseChecksums(ctx, db)
	if err != nil {
		log.Fatalf("load database checksums: %v", err)
	}

	var updated int
	for _, item := range fileChecksums {
		dbChecksum, ok := dbChecksums[item.Filename]
		if !ok || dbChecksum == item.Checksum {
			continue
		}
		if _, err := db.ExecContext(ctx,
			"UPDATE schema_migrations SET checksum = $1 WHERE filename = $2",
			item.Checksum,
			item.Filename,
		); err != nil {
			log.Fatalf("update checksum for %s: %v", item.Filename, err)
		}
		updated++
		fmt.Printf("updated %s\n", item.Filename)
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
