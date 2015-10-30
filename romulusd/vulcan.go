package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"k8s.io/kubernetes/pkg/runtime"

	gJSON "github.com/albertrdixon/gearbox/json"
	"github.com/albertrdixon/gearbox/url"
	"github.com/mailgun/vulcand/engine"
	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/plugin/registry"
)

const (
	HTTP  = "http"
	HTTPS = "https"
	WS    = "ws"
	WSS   = "wss"
)

// VulcanObject represents a vulcand component
type VulcanObject interface {
	// Key returns the etcd key for this object
	Key() string
	// Val returns the (JSON-ified) value to store in etcd
	Val() (string, error)
}

type BackendList struct {
	s map[string]string
	i map[int]string
}

func NewBackendList() *BackendList {
	return &BackendList{make(map[string]string), make(map[int]string)}
}

func (b BackendList) String() string {
	ids, sl := map[string]*struct{}{}, []string{}
	for _, v := range b.i {
		if _, ok := ids[v]; !ok {
			ids[v] = nil
			sl = append(sl, v)
		}
	}
	for _, v := range b.s {
		if _, ok := ids[v]; !ok {
			ids[v] = nil
			sl = append(sl, v)
		}
	}
	return fmt.Sprintf("Backends([%s])", strings.Join(sl, ", "))
}

func (b BackendList) Add(port int, name, bid string) {
	if name != "" {
		debugf("Adding to Backend list: [%s]: %s", name, bid)
		b.s[name] = bid
	}
	if port != 0 {
		debugf("Adding to Backend list: [%d]: %s", port, bid)
		b.i[port] = bid
	}
}

func (b BackendList) Lookup(port int, name string) (ba string, ok bool) {
	ba, ok = b.i[port]
	if ok {
		return
	}
	ba, ok = b.s[name]
	return
}

// Backend is a vulcand backend
type Backend struct {
	ID       string `json:"Id,omitempty"`
	Type     string
	Settings *BackendSettings `json:",omitempty"`
}

// BackendSettings is vulcand backend settings
type BackendSettings struct {
	Timeouts  *BackendSettingsTimeouts  `json:",omitempty"`
	KeepAlive *BackendSettingsKeepAlive `json:",omitempty"`
	TLS       *TLSSettings              `json:",omitempty"`
}

// BackendSettingsTimeouts is vulcand settings for backend timeouts
type BackendSettingsTimeouts struct {
	Read         Duration `json:",omitempty"`
	Dial         Duration `json:",omitempty"`
	TLSHandshake Duration `json:",omitempty"`
}

// BackendSettingsKeepAlive is vulcand settings for backend keep alive
type BackendSettingsKeepAlive struct {
	Period              Duration `json:",omitempty"`
	MaxIdleConnsPerHost int      `json:",omitempty"`
}

type TLSSettings struct {
	PreferServerCipherSuites bool          `json:",omitempty"`
	InsecureSkipVerify       bool          `json:",omitempty"`
	SessionTicketsDisabled   bool          `json:",omitempty"`
	SessionCache             *SessionCache `json:",omitempty"`
	CipherSuites             []string      `json:",omitempty"`
	MinVersion               string        `json:",omitempty"`
	MaxVersion               string        `json:",omitempty"`
}

type SessionCache struct {
	Type     string
	Settings *SessionCacheSettings
}

type SessionCacheSettings struct {
	Capacity int
}

// ServerMap is a map of IPs (string) -> Server
type ServerMap map[string]*Server

func (s ServerMap) String() string {
	sl := make([]string, 0, len(s))
	for k := range s {
		sl = append(sl, s[k].ID)
	}
	sort.Strings(sl)
	return fmt.Sprintf("[%s]", strings.Join(sl, ", "))
}

// Server is a vulcand server
type Server struct {
	ID      string   `json:"-"`
	URL     *url.URL `json:"URL"`
	Backend string   `json:"-"`
}

// Frontend is a vulcand frontend
type Frontend struct {
	ID        string `json:"Id,omitempty"`
	Type      string
	BackendID string `json:"BackendId"`
	Route     string
	Settings  *FrontendSettings `json:",omitempty"`
}

// FrontendSettings is vulcand frontend settings
type FrontendSettings struct {
	FailoverPredicate  string                  `json:",omitempty"`
	Hostname           string                  `json:",omitempty"`
	TrustForwardHeader bool                    `json:",omitempty"`
	Limits             *FrontendSettingsLimits `json:",omitempty"`
}

// FrontendSettingsLimits is vulcand settings for frontend limits
type FrontendSettingsLimits struct {
	MaxMemBodyBytes int
	MaxBodyBytes    int
}

type Middleware struct {
	ID       string `json:"Id"`
	Frontend string `json:"-"`
	Priority int
	Type     string
	Config   plugin.Middleware `json:"Middleware"`
}

type RawMiddleware struct {
	Type     string
	Priority int
}

type middlewareMap map[string]*Middleware

// NewBackend returns a ref to a Backend object
func NewBackend(id string) *Backend {
	return &Backend{
		ID:   id,
		Type: HTTP,
	}
}

// NewFrontend returns a ref to a Frontend object
func NewFrontend(id, bid string, route ...string) *Frontend {
	sort.StringSlice(route).Sort()
	rt := strings.Join(route, " && ")
	return &Frontend{
		ID:        id,
		BackendID: bid,
		Type:      HTTP,
		Route:     rt,
	}
}

// NewBackendSettings returns BackendSettings from raw JSON
func NewBackendSettings(p []byte) *BackendSettings {
	var ba BackendSettings
	if er := gJSON.Decode(&ba, p); er != nil {
		warnf("Failed to Marshal settings %q: %v", string(p), er)
		return nil
	}
	return &ba
}

// NewFrontendSettings returns FrontendSettings from raw JSON
func NewFrontendSettings(p []byte) *FrontendSettings {
	var f FrontendSettings
	if er := gJSON.Decode(&f, p); er != nil {
		warnf("Failed to Marshal settings %q: %v", string(p), er)
		return nil
	}
	return &f
}

func (b Backend) Key() string          { return backendf(b.ID) }
func (s Server) Key() string           { return serverf(s.Backend, s.ID) }
func (f Frontend) Key() string         { return frontendf(f.ID) }
func (f FrontendSettings) Key() string { return "" }
func (b BackendSettings) Key() string  { return "" }
func (m Middleware) Key() string       { return middlewaref(m.Frontend, m.ID) }

func (b Backend) Val() (string, error)          { return encode(b) }
func (s Server) Val() (string, error)           { return encode(s) }
func (f Frontend) Val() (string, error)         { return encode(f) }
func (f FrontendSettings) Val() (string, error) { return "", nil }
func (b BackendSettings) Val() (string, error)  { return "", nil }
func (m Middleware) Val() (string, error)       { return encode(m) }

// DirKey returns the etcd directory key for this Backend
func (b Backend) DirKey() string { return backendDirf(b.ID) }

// DirKey returns the etcd directory key for this Frontend
func (f Frontend) DirKey() string { return frontendDirf(f.ID) }

func (f *FrontendSettings) String() string {
	s, e := encode(f)
	if e != nil {
		return e.Error()
	}
	return s
}

func (b *BackendSettings) String() string {
	s, e := encode(b)
	if e != nil {
		return e.Error()
	}
	return s
}

func (s Server) String() (st string) {
	st, _ = s.Val()
	return fmt.Sprintf("Server(ID=%q, Backend=%q, URL=%v)", s.ID, s.Backend, s.URL)
}

func (f Frontend) String() (s string) {
	return fmt.Sprintf("Frontend(ID=%q, Backend=%q, Type=%q, Route=%q, Settings=%v)",
		f.ID, f.BackendID, f.Type, f.Route, f.Settings)
}

func (b Backend) String() string {
	return fmt.Sprintf("Backend(ID=%q, Type=%q, Settings=%v)", b.ID, b.Type, b.Settings)
}

func (m Middleware) String() string {
	return fmt.Sprintf("Middleware(Frontend=%q, Type=%q, Priority=%d, Config=%v)",
		m.Frontend, m.Type, m.Priority, m.Config)
}

func (m middlewareMap) String() string {
	sl := make([]string, 0, len(m))
	for k, mid := range m {
		sl = append(sl, fmt.Sprintf("%s:%s", k, mid.Type))
	}
	return fmt.Sprintf("[%s]", strings.Join(sl, ", "))
}

// IPs returns the ServerMap IPs
func (s ServerMap) IPs() []string {
	st := []string{}
	for ip := range s {
		st = append(st, ip)
	}
	return st
}

func encode(v VulcanObject) (string, error) {
	s, er := gJSON.Encode(v)
	if er != nil {
		return s, er
	}
	return strings.TrimSpace(s), nil
}

func decode(v VulcanObject, p []byte) error {
	return gJSON.Decode(v, p)
}

func buildRoute(ns string, a map[string]string) string {
	var rteConv = map[string]string{
		"host":         "Host(`%s`)",
		"method":       "Method(`%s`)",
		"path":         "Path(`%s`)",
		"header":       "Header(`%s`)",
		"hostRegexp":   "HostRegexp(`%s`)",
		"methodRegexp": "MethodRegexp(`%s`)",
		"pathRegexp":   "PathRegexp(`%s`)",
		"headerRegexp": "HeaderRegexp(`%s`)",
	}
	rt := []string{}
	if ns != "" {
		ns = fmt.Sprintf(".%s", ns)
	}
	for k, f := range rteConv {
		pk, ppk := annotationf(k, ns), annotationf(k, "")
		if v, ok := a[pk]; ok {
			if k == "method" {
				v = strings.ToUpper(v)
			}
			rt = append(rt, fmt.Sprintf(f, v))
		} else if v, ok := a[ppk]; ok {
			if k == "method" {
				v = strings.ToUpper(v)
			}
			rt = append(rt, fmt.Sprintf(f, v))
		}
	}
	if len(rt) < 1 {
		rt = []string{"Path(`/`)"}
	}
	sort.StringSlice(rt).Sort()
	return strings.Join(rt, " && ")
}

func getMiddlewares(f *Frontend, an map[string]string) map[string]*Middleware {
	ptn := `^romulus/middleware\.([^\.]+)\.?([^\.]+)?`
	mids := map[string]*Middleware{}
	for k, v := range an {
		if m := regexp.MustCompile(ptn).FindStringSubmatch(k); m != nil {
			name := m[1]
			if m[2] != "" {
				name = m[2]
				r, e := regexp.Compile("^" + m[1])
				if e != nil || !r.MatchString(f.ID) {
					continue
				}
			}
			id := md5Hash(f.ID, name)[:hashLen]
			mid := &Middleware{ID: id, Frontend: f.ID}
			if e := decode(mid, []byte(v)); e != nil {
				errorf("Failed to decode Middleware: %v", e)
				continue
			}
			mids[id] = mid
		}
	}
	return mids
}

func getVulcanID(name, ns, port string) string {
	var id []string
	if port != "" {
		id = []string{port, name, ns}
	} else {
		id = []string{name, ns}
	}
	return strings.Join(id, ".")
}

func parseVulcanID(id string) (string, string, error) {
	bits := strings.Split(id, ".")
	if len(bits) < 2 {
		return "", "", NewErr(nil, "Invalid vulcan ID %q", id)
	}
	return bits[len(bits)-2], bits[len(bits)-1], nil
}

func etcdKeyf(v string) string {
	return fmt.Sprintf("/%s", strings.Trim(v, "/"))
}

func getVulcanKey(o runtime.Object) string {
	m, er := getMeta(o)
	if er != nil {
		return *vulcanKey
	}
	if val, ok := m.labels[labelf("vulcanKey")]; ok {
		return etcdKeyf(val)
	}
	return *vulcanKey
}

func (m *Middleware) UnmarshalJSON(p []byte) error {
	mid, er := engine.MiddlewareFromJSON(p, registry.GetRegistry().GetSpec)
	if er != nil {
		return er
	}
	m.Type = mid.Type
	m.Priority = mid.Priority
	m.Config = mid.Middleware
	return nil
}
