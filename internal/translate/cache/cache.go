package cache

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

func NewDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(
		"CREATE TABLE IF NOT EXISTS cache (key TEXT PRIMARY KEY NOT NULL, timestamp INT, contents BLOB) STRICT",
	); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{
		db: db,
	}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

var ErrNoSuchKey = errors.New("no such key")

func (d *DB) Get(key string) ([]byte, error) {
	row := d.db.QueryRow("SELECT contents FROM cache WHERE key = ?", key)
	if err := row.Err(); err != nil {
		return nil, err
	}

	var blob []byte
	if err := row.Scan(&blob); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoSuchKey
		}
		return nil, err
	}

	return blob, nil
}

func (d *DB) Put(key string, timestamp time.Time, blob []byte) error {
	if _, err := d.db.Exec(
		"INSERT OR IGNORE INTO cache (key, timestamp, contents) VALUES (?, ?, ?)",
		key, timestamp.Unix(), blob,
	); err != nil {
		return err
	}
	return nil
}

func (d *DB) Touch(key string, timestamp time.Time) error {
	if _, err := d.db.Exec(
		"UPDATE cache SET timestamp = ? WHERE key = ? AND timestamp < ?",
		timestamp.Unix(), key, timestamp.Unix(),
	); err != nil {
		return err
	}
	return nil
}

func (d *DB) Clean(timestamp time.Time) error {
	if _, err := d.db.Exec("DELETE FROM cache WHERE timestamp < ?", timestamp.Unix()); err != nil {
		return err
	}
	return nil
}

type Cache struct {
	mu sync.Mutex
	db *DB
}

func NewCache(db *DB) *Cache {
	return &Cache{
		db: db,
	}
}

func (c *Cache) Get(key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	blob, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	if err := c.db.Touch(key, time.Now().Truncate(time.Hour)); err != nil {
		return nil, err
	}
	return blob, nil
}

func (c *Cache) Put(key string, blob []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.db.Put(key, time.Now().Truncate(time.Hour), blob)
}

func (c *Cache) Clean() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.db.Clean(time.Now().Truncate(time.Hour).Add(-7 * 24 * time.Hour))
}

// Delete allows a corrupt immutable entry to be replaced on the next miss.
func (d *DB) Delete(key string) error {
	_, err := d.db.Exec("DELETE FROM cache WHERE key = ?", key)
	return err
}

func (c *Cache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.db.Delete(key)
}
