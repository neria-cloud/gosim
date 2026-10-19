//go:build !race

package compose

func isRaceBuild() bool { return false }
