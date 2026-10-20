package behavior

const (
	initialPublicGlobal  = 20
	initialPrivateGlobal = 10
)

var privateGlobal int

var PublicGlobal int

func init() {
	PublicGlobal = initialPublicGlobal
	privateGlobal = initialPrivateGlobal
}
