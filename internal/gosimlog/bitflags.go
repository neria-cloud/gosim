package gosimlog

import (
	"bytes"
	"strconv"
)

const bitflagSeparator = "|"

type BitflagChoice struct {
	Mask   int
	Values map[int]string
}

type BitflagValue struct {
	Value int
	Name  string
}

type BitflagFormatter struct {
	Choices []BitflagChoice
	Flags   []BitflagValue
}

func (f *BitflagFormatter) Format(value int) string {
	var buf bytes.Buffer
	appendPart := func(part string) {
		if buf.Len() > 0 {
			buf.WriteString(bitflagSeparator)
		}
		buf.WriteString(part)
	}
	remaining := value
	for _, choice := range f.Choices {
		masked := remaining & choice.Mask
		remaining ^= masked
		if got, ok := choice.Values[masked]; ok {
			appendPart(got)
		} else {
			appendPart(strconv.Itoa(masked))
		}
	}
	for _, choice := range f.Flags {
		if remaining&choice.Value == choice.Value {
			remaining ^= choice.Value
			appendPart(choice.Name)
		}
	}
	if remaining != 0 {
		appendPart(strconv.Itoa(remaining))
	}
	return buf.String()
}
