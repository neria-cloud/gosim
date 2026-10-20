//go:build sim

package bolt_test

import (
	"fmt"
	"log"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestBbolt(t *testing.T) {
	path := "test.db"
	db, err := bolt.Open(path, 0o666, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucket([]byte("bucket1"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte("key1"), []byte("value"))
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("bucket1"))
		value := bucket.Get([]byte("key1"))
		if string(value) != "value" {
			t.Errorf("bad read %q", string(value))
		}
		log.Println(value)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("bucket1"))
		for i := 0; i < 100; i++ {
			if err := bucket.Put([]byte(fmt.Sprintf("key%d", i)), []byte(fmt.Sprint(i*i))); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = bolt.Open(path, 0o666, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("bucket1"))
		for i := 0; i < 100; i++ {
			value := bucket.Get([]byte(fmt.Sprintf("key%d", i)))
			if string(value) != fmt.Sprint(i*i) {
				t.Errorf("bad read %q", string(value))
			}
			log.Println(value)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
