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
			r["host"] = types.Route{Rule: "Host", Value: part.Value()}
		case kubernetes.MethodPart:
			r["methods"] = types.Route{Rule: "Methods", Value: part.Value()}
		case kubernetes.PathPart:
			r["path"] = types.Route{Rule: "Path", Value: part.Value()}
		case kubernetes.PrefixPart:
			r["prefix"] = types.Route{Rule: "PathPrefix", Value: part.Value()}
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
		r["headers"] = types.Route{Rule: "Headers", Value: strings.Join(headers, ", ")}
	}
	if len(headersRgx) > 0 {
		r["headersRegexp"] = types.Route{Rule: "HeadersRegexp", Value: strings.Join(headersRgx, ", ")}
	}

	return r
}
