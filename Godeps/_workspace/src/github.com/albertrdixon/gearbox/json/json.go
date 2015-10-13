// Package json provides a simple wrapper around the json library in order to reverse
// the HTML escaping the main json lib does.
package json

import (
	"bytes"
	"encoding/json"
	"strings"
)

var unicodeReplacements = map[string]string{
	`\u003c`: "<",
	`\u003e`: ">",
	`\u0026`: "&",
}

type Reader interface {
	Read(p []byte) (n int, err error)
}

// HTMLUnescape reverses the HTMLEscape process done by JSON encoding
func HTMLUnescape(s string) string {
	r := s
	for k, v := range unicodeReplacements {
		r = strings.Replace(r, k, v, -1)
	}
	return r
}

func Encode(o interface{}) (string, error) {
	b := new(bytes.Buffer)
	if e := json.NewEncoder(b).Encode(o); e != nil {
		return b.String(), e
	}
	return HTMLUnescape(b.String()), nil
}

func Decode(o interface{}, p []byte) error {
	b := bytes.NewBuffer(p)
	return DecodeStream(o, b)
}

func DecodeStream(o interface{}, r Reader) error {
	return json.NewDecoder(r).Decode(o)
}
