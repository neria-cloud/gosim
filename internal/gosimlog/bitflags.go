package gosimlog

import (
	"bytes"
	"fmt"
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
	remaining := value
	for _, choice := range f.Choices {
		masked := remaining & choice.Mask
		remaining ^= masked
		if got, ok := choice.Values[masked]; ok {
			if buf.Len() > 0 {
				buf.WriteString(bitflagSeparator)
			}
			buf.WriteString(got)
		} else {
			if buf.Len() > 0 {
				buf.WriteString(bitflagSeparator)
			}
			fmt.Fprintf(&buf, "%d", masked)
		}
	}
	for _, choice := range f.Flags {
		if remaining&choice.Value == choice.Value {
			remaining ^= choice.Value
			if buf.Len() > 0 {
				buf.WriteString(bitflagSeparator)
			}
			buf.WriteString(choice.Name)
		}
	}
	if remaining != 0 {
		if buf.Len() > 0 {
			buf.WriteString(bitflagSeparator)
		}
		fmt.Fprintf(&buf, "%d", remaining)
	}
	return buf.String()
}
