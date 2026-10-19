package cache_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neria-cloud/gosim/internal/translate/cache"
)

func TestDeleteLastUseRetentionAndUnchangedSchema(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache.sqlite")
	db, err := cache.NewDB(file)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := cache.NewCache(db)
	now := time.Now().Truncate(time.Hour)
	for _, key := range []string{"old", "used"} {
		if err := db.Put(key, now.Add(-8*24*time.Hour), []byte(key)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Put("recent", now.Add(-6*24*time.Hour), []byte("recent")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get("used"); err != nil {
		t.Fatal(err)
	}
	if err := c.Clean(); err != nil {
		t.Fatal(err)
	}
	mustMiss(t, db, "old")
	mustGet(t, db, "used", "used")
	mustGet(t, db, "recent", "recent")
	if err := c.Delete("used"); err != nil {
		t.Fatal(err)
	}
	mustMiss(t, db, "used")
	if err := c.Delete("used"); err != nil {
		t.Fatal("delete must be idempotent", err)
	}
	if err := c.Put("used", []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	mustGet(t, db, "used", "replacement")
	audit, err := sql.Open("sqlite3", file)
	if err != nil {
		t.Fatal(err)
	}
	defer audit.Close()
	var timestamp int64
	if err := audit.QueryRow("SELECT timestamp FROM cache WHERE key = ?", "used").Scan(&timestamp); err != nil {
		t.Fatal(err)
	}
	if timestamp%3600 != 0 || timestamp < now.Unix() || timestamp > time.Now().Truncate(time.Hour).Unix() {
		t.Fatal("timestamp is not coarsened last use", timestamp)
	}
	rows, err := audit.Query("SELECT name, sql FROM sqlite_master WHERE type = 'table'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		count++
		if name != "cache" || !strings.HasSuffix(ddl, "STRICT") || !strings.Contains(ddl, "key TEXT PRIMARY KEY NOT NULL") {
			t.Fatal("persistent schema changed", name, ddl)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("additional persistent tables", count)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete("used"); err == nil || errors.Is(err, cache.ErrNoSuchKey) {
		t.Fatal("delete database error hidden", err)
	}
}
