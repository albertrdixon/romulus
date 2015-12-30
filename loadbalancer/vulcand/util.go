package vulcand

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/albertrdixon/gearbox/logger"
	"github.com/albertrdixon/gearbox/url"
	"github.com/bradfitz/slice"
	"github.com/mailgun/route"
	"github.com/timelinelabs/romulus/kubernetes"
)

func buildRoute(rt *kubernetes.Route) string {
	if rt.Empty() {
		return DefaultRoute
	}

	bits := []string{}
	for k, v := range rt.GetParts() {
		bits = append(bits, fmt.Sprintf("%s(`%s`)", strings.Title(k), v))
	}
	for k, v := range rt.GetHeader() {
		bits = append(bits, fmt.Sprintf("Header(`%s`, `%s`)", k, v))
	}
	slice.Sort(bits, func(i, j int) bool {
		return bits[i] < bits[j]
	})
	expr := strings.Join(bits, " && ")
	if len(expr) < 1 || !route.IsValid(expr) {
		logger.Debugf("Provided route not valid: %s", expr)
		return DefaultRoute
	}
	return expr
}

func isRegexp(r string) bool {
	if !strings.HasPrefix(r, "/") || !strings.HasSuffix(r, "/") {
		return false
	}
	if _, er := regexp.Compile(r); er != nil {
		logger.Debugf("Regexp compile failure: %v", er)
		return false
	}
	return true
}

func validVulcanURL(u *url.URL) bool {
	return true
	// return len(u.GetHost()) > 0 && len(u.Scheme) > 0 && len(u.Path) > 0
}
