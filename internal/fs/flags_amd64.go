// Linux O_DIRECTORY follows the simulated architecture, independently of host OS.
package fs

const (
	openDirectory = 0x10000
	openNoFollow  = 0x20000
)
