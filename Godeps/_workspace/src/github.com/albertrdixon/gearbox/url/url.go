// Package url wraps the net/url package so that the url.URL type is Marshallable
package url

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
)

// URL is an Unmarshallable url type
type URL url.URL

// UnmarshalJSON parses JSON string into url.URL
func (u *URL) UnmarshalJSON(p []byte) error {
	nu, err := url.Parse(string(bytes.Trim(p, `"`)))
	if err != nil {
		return err
	}
	(*u) = URL(*nu)
	return nil
}

// MarshalJSON turns url into a JSON string
func (u *URL) MarshalJSON() ([]byte, error) {
	s := fmt.Sprintf(`"%s"`, u.String())
	return []byte(s), nil
}

// GetPath returns url.Path with leading '/' removed
func (u *URL) GetPath() string {
	if u == nil {
		return ""
	}
	return strings.TrimLeft(u.Path, "/")
}

// GetHost url.Host without the port suffix
func (u *URL) GetHost() string {
	if u == nil {
		return ""
	}
	i := strings.Index(u.Host, ":")
	if i == -1 {
		return u.Host
	}
	return u.Host[0:i]
}

// String returns the string representation
func (u *URL) String() string {
	return (*url.URL)(u).String()
}
