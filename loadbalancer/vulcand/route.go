package vulcand

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/timelinelabs/romulus/kubernetes"
)

const (
	HostPart        = "Host"
	HostRegexPart   = "HostRegexp"
	PathPart        = "Path"
	PathRegexPart   = "PathRegexp"
	MethodPart      = "Method"
	MethodRegexPart = "MethodRegexp"
	HeaderPart      = "Header"
	HeaderRegexPart = "HeaderRegexp"
)

func NewRoute(rt *kubernetes.Route) *route {
	var (
		r      = &route{methods: make([]*routePart, 0, 1), headers: make([]*routePart, 0, 1)}
		prefix = false
	)

	if rt.Empty() {
		return &route{path: &routePart{part: PathRegexPart, val: ".*"}}
	}

	for _, part := range rt.Parts() {
		rp := &routePart{val: part.Value()}
		switch part.Type() {
		case kubernetes.HostPart:
			// r.host = &routePart{part: "Host", val: part.Value(), regex: part.IsRegex()}
			if part.IsRegex() {
				rp.part = HostRegexPart
			} else {
				rp.part = HostPart
			}
			r.host = rp
		case kubernetes.MethodPart:
			if part.IsRegex() {
				rp.part = MethodRegexPart
			} else {
				rp.part = MethodPart
			}
			r.methods = append(r.methods, rp)
		case kubernetes.HeaderPart:
			if part.IsRegex() {
				rp.part = HeaderRegexPart
			} else {
				rp.part = HeaderPart
			}
			rp.header = part.Header()
			r.headers = append(r.headers, rp)
		case kubernetes.PathPart:
			if part.IsRegex() {
				rp.part = PathRegexPart
			} else {
				rp.part = PathPart
			}
			if !prefix {
				r.path = rp
			}
		case kubernetes.PrefixPart:
			rp.part = PathRegexPart
			rp.val = fmt.Sprintf("%s.*", part.Value())
			r.path = rp
			prefix = true
		}
	}

	return r
}

func NewRouteFromString(expr string) *route {
	var (
		r       = &route{methods: make([]*routePart, 0, 1), headers: make([]*routePart, 0, 1)}
		matcher = regexp.MustCompile(`(\w+)\((.+)\)`)
		list    = strings.FieldsFunc(expr, func(r rune) bool {
			return unicode.IsSpace(r) || r == '&'
		})
	)

	for _, bit := range list {
		if m := matcher.FindAllStringSubmatch(bit, -1); m != nil && len(m[0]) == 2 {
			part := &routePart{part: m[0][0]}
			value := strings.SplitN(strings.Trim(m[0][1], "`"), "`, `", 2)
			if len(value) == 2 {
				part.header = value[0]
				part.val = value[1]
			} else {
				part.val = value[0]
			}
		}
	}
	return r
}

func (r *route) String() string {
	var (
		parts = make([]string, 0, 1)
	)

	if r.host != nil {
		parts = append(parts, r.host.String())
	}
	if r.path != nil {
		parts = append(parts, r.path.String())
	}
	for _, method := range r.methods {
		parts = append(parts, method.String())
	}
	for _, header := range r.headers {
		parts = append(parts, header.String())
	}

	return strings.Join(parts, " && ")
}

func (r *routePart) String() string {
	var val string

	// if r.regex {
	// 	kind = fmt.Sprintf("%sRegexp", r.part)
	// } else {
	// 	kind = r.part
	// }

	if r.header != "" {
		val = fmt.Sprintf("`%s`, `%s`", r.header, r.val)
	} else {
		val = fmt.Sprintf("`%s`", r.val)
	}

	return fmt.Sprintf("%s(%s)", r.part, val)
}
