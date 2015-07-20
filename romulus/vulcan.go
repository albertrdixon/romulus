package romulus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
)

var (
	bckndDirFmt  = "/vulcan/backends/%s"
	frntndDirFmt = "/vulcan/frontends/%s"
	bckndFmt     = "/vulcan/backends/%s/backend"
	srvrDirFmt   = "/vulcan/backends/%s/servers"
	srvrFmt      = "/vulcan/backends/%s/servers/%s"
	frntndFmt    = "/vulcan/frontends/%s/frontend"
)

// VulcanObject represents a vulcand component
type VulcanObject interface {
	// Key returns the etcd key for this object
	Key() string
	// Val returns the (JSON-ified) value to store in etcd
	Val() (string, error)
}

// Backend is a vulcand backend
type Backend struct {
	ID       uuid.UUID `json:"-"`
	Type     string
	Settings *BackendSettings `json:",omitempty"`
}

// BackendSettings is vulcand backend settings
type BackendSettings struct {
	Timeouts  *BackendSettingsTimeouts  `json:",omitempty"`
	KeepAlive *BackendSettingsKeepAlive `json:",omitempty"`
}

// BackendSettingsTimeouts is vulcand settings for backend timeouts
type BackendSettingsTimeouts struct {
	Read         time.Duration `json:",omitempty"`
	Dial         time.Duration `json:",omitempty"`
	TLSHandshake time.Duration `json:",omitempty"`
}

// BackendSettingsKeepAlive is vulcand settings for backend keep alive
type BackendSettingsKeepAlive struct {
	Period              time.Duration `json:",omitempty"`
	MaxIdleConnsPerHost int           `json:",omitempty"`
}

// ServerMap is a map of IPs (string) -> Server
type ServerMap map[string]Server

// Server is a vulcand server
type Server struct {
	URL     *URL      `json:"URL"`
	Backend uuid.UUID `json:"-"`
}

// Frontend is a vulcand frontend
type Frontend struct {
	ID        uuid.UUID `json:"-"`
	Type      string
	BackendID uuid.UUID `json:"BackendId"`
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

// NewBackendSettings returns BackendSettings from raw JSON
func NewBackendSettings(p []byte) *BackendSettings {
	var ba BackendSettings
	b := bytes.NewBuffer(p)
	json.NewDecoder(b).Decode(&ba)
	return &ba
}

// NewFrontendSettings returns FrontendSettings from raw JSON
func NewFrontendSettings(p []byte) *FrontendSettings {
	var f FrontendSettings
	b := bytes.NewBuffer(p)
	json.NewDecoder(b).Decode(&f)
	return &f
}

func (b Backend) Key() string          { return fmt.Sprintf(bckndFmt, b.ID.String()) }
func (s Server) Key() string           { return fmt.Sprintf(srvrFmt, s.Backend.String(), s.URL.GetHost()) }
func (f Frontend) Key() string         { return fmt.Sprintf(frntndFmt, f.ID.String()) }
func (f FrontendSettings) Key() string { return "" }
func (b BackendSettings) Key() string  { return "" }

func (b Backend) Val() (string, error)          { return encode(b) }
func (s Server) Val() (string, error)           { return encode(s) }
func (f Frontend) Val() (string, error)         { return encode(f) }
func (f FrontendSettings) Val() (string, error) { return "", nil }
func (b BackendSettings) Val() (string, error)  { return "", nil }

// DirKey returns the etcd directory key for this Backend
func (b Backend) DirKey() string { return fmt.Sprintf(bckndDirFmt, b.ID.String()) }

// DirKey returns the etcd directory key for this Frontend
func (f Frontend) DirKey() string { return fmt.Sprintf(frntndDirFmt, f.ID.String()) }

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

// IPs returns the ServerMap IPs
func (s ServerMap) IPs() []string {
	st := []string{}
	for ip := range s {
		st = append(st, ip)
	}
	return st
}

func encode(v VulcanObject) (string, error) {
	b := new(bytes.Buffer)
	e := json.NewEncoder(b).Encode(v)
	return strings.TrimSpace(HTMLUnescape(b.String())), e
}

func buildRoute(a map[string]string) string {
	rt := []string{}
	for k, f := range routeAnnotations {
		if v, ok := a[k]; ok {
			if k == "method" {
				v = strings.ToUpper(v)
			}
			rt = append(rt, fmt.Sprintf(f, v))
		}
	}
	if len(rt) < 1 {
		rt = []string{"Path('/')"}
	}
	return strings.Join(rt, " && ")
}
