package auth

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/codegangsta/cli"
	"github.com/timelinelabs/vulcand/plugin"
	"github.com/vulcand/oxy/utils"
)

const Type = "auth"

type Auth struct {
	User, Pass string
}

type handler struct {
	Auth
	next http.Handler
	err  utils.ErrorHandler
}

func CliFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:  "user, u",
			Usage: "Basic auth username",
		},
		cli.StringFlag{
			Name:  "pass, p",
			Usage: "Basic auth password",
		},
	}
}

func GetSpec() *plugin.MiddlewareSpec {
	return &plugin.MiddlewareSpec{
		Type:      Type,
		FromOther: FromOther,
		FromCli:   FromCli,
		CliFlags:  CliFlags(),
	}
}

func New(user, pass string) (*Auth, error) {
	if len(user) < 1 || len(pass) < 1 {
		return nil, errors.New("Neither username nor password may be empty!")
	}
	return &Auth{user, pass}, nil
}

func (a *Auth) NewHandler(next http.Handler) (http.Handler, error) {
	return &handler{
		Auth: *a,
		next: next,
	}, nil
}

func (a *Auth) String() string {
	return fmt.Sprintf("username=%s password=******", a.User)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	auth, er := utils.ParseAuthHeader(r.Header.Get("Authorization"))
	if er != nil || !authorized(h.User, h.Pass, auth) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(http.StatusText(http.StatusForbidden)))
		return
	}
	h.next.ServeHTTP(w, r)
}

func FromOther(a Auth) (plugin.Middleware, error) {
	return New(a.User, a.Pass)
}

func FromCli(c *cli.Context) (plugin.Middleware, error) {
	return New(c.String("user"), c.String("pass"))
}

func authorized(u, p string, a *utils.BasicAuth) bool {
	return u == a.Username && p == a.Password
}
