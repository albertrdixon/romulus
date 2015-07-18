package romulus

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
)

// URL is an Unmarshallable url type
type URL url.URL

type RawString string

// UnmarshalJSON parses JSON string into url.URL
func (u *URL) UnmarshalJSON(p []byte) error {
	nu, err := url.Parse(string(bytes.Trim(p, "\"")))
	if err != nil {
		return err
	}
	(*u) = URL(*nu)
	return nil
}

// GetPath returns url.Path with leading '/' removed
func (u *URL) GetPath() string {
	return strings.TrimLeft(u.Path, "/")
}

// String returns the string representation
func (u *URL) String() string {
	return (*url.URL)(u).String()
}

func (r RawString) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf(`"%s"`, r)
	return []byte(s), nil
}
