//go:build go1.25 && !go1.26

package reflect

import "reflect" //gosim:notranslate

func Swapper(slice any) func(i, j int) {
	return reflect.Swapper(slice)
}
