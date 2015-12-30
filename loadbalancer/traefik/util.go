package traefik

import (
	"github.com/emilevauge/traefik/types"
	"github.com/timelinelabs/romulus/kubernetes"
)

func buildRoute(rt *kubernetes.Route) map[string]types.Route {
	if rt.Empty() {
		return defaultTraefikRoute
	}

	routes := map[string]types.Route{}
	for rule, val := range rt.GetParts() {
		routes[rule] = types.Route{Rule: rule, Value: val}
	}
	return routes
}
