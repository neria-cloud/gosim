//go:build go1.26

package reflect

import "iter" //gosim:notranslate

type typeExtension interface {
	Fields() iter.Seq[StructField]
	Methods() iter.Seq[Method]
	Ins() iter.Seq[Type]
	Outs() iter.Seq[Type]
}
