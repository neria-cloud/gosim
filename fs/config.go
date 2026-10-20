// Package fs controls the optional simulated filesystem provider.
package fs

type Config struct{ WorkingDir string }

func DefaultConfig() Config { return Config{WorkingDir: "/app"} }
