package buildcfg

const (
	fixtureDigestLength  = 64
	fixtureDirPerm       = 0o700
	fixtureExecPerm      = 0o700
	fixtureFilePerm      = 0o600
	fixtureSharedDirPerm = 0o755

	featureFS  = "fs"
	featureNet = "net"
	featureRT  = "rt"
	tagSim     = "sim"
)
