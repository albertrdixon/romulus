package vulcand

import (
	"fmt"
	"strings"

	"github.com/timelinelabs/romulus/kubernetes"
)

func NewRoute(rt *kubernetes.Route) *route {
	var (
		r      = &route{method: make([]*routePart, 0, 1), header: make([]*routePart, 0, 1)}
		prefix = false
	)

	if rt.Empty() {
		return &route{path: &routePart{part: "Path", val: ".*", regex: true}}
	}

	for _, part := range rt.Parts() {
		switch part.Type() {
		case kubernetes.HostPart:
			r.host = &routePart{part: "Host", val: part.Value(), regex: part.IsRegex()}
		case kubernetes.MethodPart:
			r.method = append(
				r.method,
				&routePart{part: "Method", val: part.Value(), regex: part.IsRegex()},
			)
		case kubernetes.HeaderPart:
			r.header = append(
				r.header,
				&routePart{part: "Header", header: part.Header(), val: part.Value(), regex: part.IsRegex()},
			)
		case kubernetes.PathPart:
			if !prefix {
				r.path = &routePart{part: "Path", val: part.Value(), regex: part.IsRegex()}
			}
		case kubernetes.PrefixPart:
			val := fmt.Sprintf("%s.*", part.Value())
			r.path = &routePart{part: "Path", val: val, regex: true}
			prefix = true
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
	for _, method := range r.method {
		parts = append(parts, method.String())
	}
	for _, header := range r.header {
		parts = append(parts, header.String())
	}

	return strings.Join(parts, " && ")
}

func (r *routePart) String() string {
	var (
		kind, val string
	)

	if r.regex {
		kind = fmt.Sprintf("%sRegexp", r.part)
	} else {
		kind = r.part
	}

	if r.header != "" {
		val = fmt.Sprintf("`%s`, `%s`", r.header, r.val)
	} else {
		val = fmt.Sprintf("`%s`", r.val)
	}

	return fmt.Sprintf("%s(%s)", kind, val)
}
