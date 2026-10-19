//go:build go1.26 && !go1.27

package reflect

import "reflect" //gosim:notranslate

func VisibleFields(t Type) []StructField {
	return wrapStructFields(reflect.VisibleFields(t.(*typeImpl).inner))
}
