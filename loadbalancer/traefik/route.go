package traefik

import (
	"fmt"
	"strings"

	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/romulus/kubernetes"
)

func NewRoute(rt *kubernetes.Route) map[string]types.Route {
	var (
		r          = make(map[string]types.Route)
		headers    = make([]string, 0, 1)
		headersRgx = make([]string, 0, 1)
	)

	for _, part := range rt.Parts() {
		switch part.Type() {
		case kubernetes.HostPart:
			r["host"] = types.Route{Rule: fmt.Sprintf("Host: %s", part.Value())}
		case kubernetes.MethodPart:
			r["methods"] = types.Route{Rule: fmt.Sprintf("Methods: %s", part.Value())}
		case kubernetes.PathPart:
			r["path"] = types.Route{Rule: fmt.Sprintf("Path: %s", part.Value())}
		case kubernetes.PrefixPart:
			r["prefix"] = types.Route{Rule: fmt.Sprintf("PathPrefix: %s", part.Value())}
		case kubernetes.HeaderPart:
			head := fmt.Sprintf("%q, %q", part.Header(), part.Value())
			if part.IsRegex() {
				headersRgx = append(headersRgx, head)
			} else {
				headers = append(headers, head)
			}
		}
	}

	if len(headers) > 0 {
		r["headers"] = types.Route{Rule: fmt.Sprintf("Headers: %s", strings.Join(headers, ", "))}
	}
	if len(headersRgx) > 0 {
		r["headersRegexp"] = types.Route{Rule: fmt.Sprintf("HeadersRegexp: %s", strings.Join(headersRgx, ", "))}
	}

	return r
}
