package nginx

import (
	"net/url"

	"golang.org/x/net/context"
)

type nginx struct {
	context.Context
	data
}

type data struct {
	frontends map[string]frontend
	backends  map[string]backend
}

type frontend struct {
	ID string
}

type backend struct {
	ID      string
	Servers []server
}

type server struct {
	ID  string
	URL *url.URL
}
