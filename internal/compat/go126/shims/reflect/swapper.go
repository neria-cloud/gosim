//go:build go1.26 && !go1.27

package reflect

import "reflect" //gosim:notranslate

func Swapper(slice any) func(i, j int) {
	return reflect.Swapper(slice)
}
