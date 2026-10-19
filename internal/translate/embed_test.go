package translate

import (
	"reflect"
	"testing"
)

func TestEmbedPatternsAndSelection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		text string
		want []string
	}{
		{"asset.txt", []string{"asset.txt"}},
		{"\"space file.txt\" `raw file.txt` assets/*", []string{"space file.txt", "raw file.txt", "assets/*"}},
	} {
		got, err := embedPatterns(test.text)
		if err != nil || !reflect.DeepEqual(got, test.want) {
			t.Fatal(got, err)
		}
	}
	for _, text := range []string{"\"unterminated", "\"one\"two"} {
		if _, err := embedPatterns(text); err == nil {
			t.Fatal("accepted invalid pattern", text)
		}
	}
	for _, test := range []struct {
		pattern, file string
		want          bool
	}{
		{"asset.txt", "asset.txt", true},
		{"*.txt", "asset.txt", true},
		{"assets", "assets/nested/file.txt", true},
		{"assets", "assets/.hidden/file.txt", false},
		{"all:assets", "assets/.hidden/file.txt", true},
		{"assets/_*", "assets/_hidden", true},
		{"assets", "other.txt", false},
	} {
		got, err := embedMatches(test.pattern, test.file)
		if err != nil || got != test.want {
			t.Fatalf("%+v: %v %v", test, got, err)
		}
	}
	for _, pattern := range []string{"../asset", "/asset", "[", "."} {
		if _, err := embedMatches(pattern, "asset"); err == nil {
			t.Fatal("invalid pattern accepted", pattern)
		}
	}
}
