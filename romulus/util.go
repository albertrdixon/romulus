package romulus

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"
)

var unicodeReplacements = map[string]string{
	`\u003c`: "<",
	`\u003e`: ">",
	`\u0026`: "&",
}

// HTMLUnescape reverses the HTMLEscape process done by JSON encoding
func HTMLUnescape(s string) string {
	r := s
	for k, v := range unicodeReplacements {
		r = strings.Replace(r, k, v, -1)
	}
	return r
}

func md5Hash(ss ...interface{}) string {
	if len(ss) < 1 {
		return ""
	}

	h := md5.New()
	for i := range ss {
		io.WriteString(h, fmt.Sprintf("%v", ss[i]))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
