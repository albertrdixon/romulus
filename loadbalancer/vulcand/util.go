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

func buildRoute(r *kubernetes.Resource) string {
	rt := r.Route
	if rt.Empty() {
		return DefaultRoute
	}

	bits, seen := []string{}, map[string]bool{}
	for k, v := range rt.GetParts() {
		part := fmt.Sprintf("%s(`%s`)", strings.Title(k), v)
		logger.Debugf("[%v] Adding %s to route", r.ID(), part)
		bits = append(bits, part)
		seen[k] = true
	}
	for k, v := range rt.GetRegex() {
		if _, ok := seen[k]; !ok {
			part := fmt.Sprintf("%sRegexp(`%s`)", strings.Title(k), v)
			logger.Debugf("[%v] Adding %s to route", r.ID(), part)
			bits = append(bits, part)
		}
	}
	for k, v := range rt.GetHeader() {
		part := fmt.Sprintf("Header(`%s`, `%s`)", k, v)
		logger.Debugf("[%v] Adding %s to route", r.ID(), part)
		bits = append(bits, part)
	}
	slice.Sort(bits, func(i, j int) bool {
		return bits[i] < bits[j]
	})
	expr := strings.Join(bits, " && ")
	if len(expr) < 1 || !route.IsValid(expr) {
		logger.Debugf("[%v] Provided route not valid: %s", r.ID(), expr)
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
