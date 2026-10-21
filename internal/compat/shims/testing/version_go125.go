//go:build go1.25 && !go1.26

package testing

type tbExtension interface{}

func testTempRoot() string { return "" }
