//go:build go1.25

// Package elliptic preserves the optional curve declarations used by etcd's
// certificate branch. Curve arithmetic and key generation are not modeled.
package elliptic

import (
	"io"
	"math/big"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

func unsupported(operation string) {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "crypto/elliptic." + operation, Message: "optional elliptic-curve operations are not modeled"})
}

type Curve interface {
	Params() *CurveParams
	IsOnCurve(x, y *big.Int) bool
	Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int)
	Double(x1, y1 *big.Int) (*big.Int, *big.Int)
	ScalarMult(x, y *big.Int, k []byte) (*big.Int, *big.Int)
	ScalarBaseMult(k []byte) (*big.Int, *big.Int)
}

type CurveParams struct {
	P, N, B, Gx, Gy *big.Int
	BitSize         int
	Name            string
}

// TODO(not planned): implement curve arithmetic only with a deterministic,
// translated computation contract and tests for the selected Go families.
func P224() Curve { unsupported("P224"); panic("abort returned") }
func P256() Curve { unsupported("P256"); panic("abort returned") }
func P384() Curve { unsupported("P384"); panic("abort returned") }
func P521() Curve { unsupported("P521"); panic("abort returned") }

func (c *CurveParams) Params() *CurveParams { return c }
func (c *CurveParams) IsOnCurve(x, y *big.Int) bool {
	unsupported("CurveParams.IsOnCurve")
	panic("abort returned")
}

func (c *CurveParams) Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	unsupported("CurveParams.Add")
	panic("abort returned")
}

func (c *CurveParams) Double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	unsupported("CurveParams.Double")
	panic("abort returned")
}

func (c *CurveParams) ScalarMult(x, y *big.Int, k []byte) (*big.Int, *big.Int) {
	unsupported("CurveParams.ScalarMult")
	panic("abort returned")
}

func (c *CurveParams) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	unsupported("CurveParams.ScalarBaseMult")
	panic("abort returned")
}

func GenerateKey(curve Curve, rand io.Reader) ([]byte, *big.Int, *big.Int, error) {
	unsupported("GenerateKey")
	panic("abort returned")
}
