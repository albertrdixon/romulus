package util

import (
	"fmt"
	"hash"
	"io"
)

func Hashf(h hash.Hash, ss ...interface{}) string {
	if len(ss) < 1 {
		return ""
	}

	for i := range ss {
		io.WriteString(h, fmt.Sprintf("%v", ss[i]))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
