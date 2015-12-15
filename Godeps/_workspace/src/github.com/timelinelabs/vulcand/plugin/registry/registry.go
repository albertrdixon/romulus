// This file will be generated to include all customer specific middlewares
package registry

import (
	"github.com/timelinelabs/vulcand/plugin"
	"github.com/timelinelabs/vulcand/plugin/auth"
	"github.com/timelinelabs/vulcand/plugin/cbreaker"
	"github.com/timelinelabs/vulcand/plugin/connlimit"
	"github.com/timelinelabs/vulcand/plugin/ratelimit"
	"github.com/timelinelabs/vulcand/plugin/rewrite"
	"github.com/timelinelabs/vulcand/plugin/trace"
)

func GetRegistry() *plugin.Registry {
	r := plugin.NewRegistry()

	specs := []*plugin.MiddlewareSpec{
		ratelimit.GetSpec(),
		connlimit.GetSpec(),
		rewrite.GetSpec(),
		cbreaker.GetSpec(),
		trace.GetSpec(),
		auth.GetSpec(),
	}

	for _, spec := range specs {
		if err := r.AddSpec(spec); err != nil {
			panic(err)
		}
	}

	return r
}
