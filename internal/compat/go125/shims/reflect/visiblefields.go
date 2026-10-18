//go:build go1.25 && !go1.26

package reflect

import "reflect" //gosim:notranslate

func VisibleFields(t Type) []StructField {
	return wrapStructFields(reflect.VisibleFields(t.(*typeImpl).inner))
}
