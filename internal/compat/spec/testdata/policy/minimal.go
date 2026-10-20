package fixture

import (
	"context"
	"crypto/rand"
	"io"
	"math/big"
	"net"
	"os"
	"testing"
	"time"
)

var (
	_ context.Context
	_ io.Reader
	_ *big.Int
	_ net.Conn
	_ *os.File
	_ *testing.T
	_ time.Time
	_ = rand.Reader
)
