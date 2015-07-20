package romulus

import (
	"strings"

	l "github.com/Sirupsen/logrus"
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

var pkgField = l.Fields{"pkg": "romulus"}

func log() *l.Entry { return logf(nil) }
func logf(f F) *l.Entry {
	fi := l.Fields{}
	for k, v := range pkgField {
		fi[k] = v
	}
	for k, v := range f {
		fi[k] = v
	}
	return l.WithFields(fi)
}

// LogLevel sets the logging level
func LogLevel(lv string) {
	if lvl, e := l.ParseLevel(lv); e == nil {
		l.SetLevel(lvl)
	}
}
