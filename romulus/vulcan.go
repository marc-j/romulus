package romulus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	TypeHTTP = "http"

	bcknds       = "backends"
	frntnds      = "frontends"
	bckndDirFmt  = "backends/%s"
	frntndDirFmt = "frontends/%s"
	bckndFmt     = "backends/%s/backend"
	srvrDirFmt   = "backends/%s/servers"
	srvrFmt      = "backends/%s/servers/%s"
	frntndFmt    = "frontends/%s/frontend"

	annotationFmt = "romulus/%s%s"
	rteConv       = map[string]string{
		"host":         "Host(`%s`)",
		"method":       "Method(`%s`)",
		"path":         "Path(`%s`)",
		"header":       "Header(`%s`)",
		"hostRegexp":   "HostRegexp(`%s`)",
		"methodRegexp": "MethodRegexp(`%s`)",
		"pathRegexp":   "PathRegexp(`%s`)",
		"headerRegexp": "HeaderRegexp(`%s`)",
	}
)

// VulcanObject represents a vulcand component
type VulcanObject interface {
	// Key returns the etcd key for this object
	Key() string
	// Val returns the (JSON-ified) value to store in etcd
	Val() (string, error)
}

type BackendList struct {
	s map[string]*Backend
	i map[int]*Backend
}

func NewBackendList() *BackendList {
	return &BackendList{make(map[string]*Backend), make(map[int]*Backend)}
}
func (b BackendList) Add(port int, name string, ba *Backend) {
	b.s[name] = ba
	b.i[port] = ba
}

func (b BackendList) Lookup(port int, name string) (ba *Backend, ok bool) {
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
	Read         time.Duration `json:",omitempty"`
	Dial         time.Duration `json:",omitempty"`
	TLSHandshake time.Duration `json:",omitempty"`
}

// BackendSettingsKeepAlive is vulcand settings for backend keep alive
type BackendSettingsKeepAlive struct {
	Period              time.Duration `json:",omitempty"`
	MaxIdleConnsPerHost int           `json:",omitempty"`
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

// Server is a vulcand server
type Server struct {
	ID      string `json:"-"`
	URL     *URL   `json:"URL"`
	Backend string `json:"-"`
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

// NewBackend returns a ref to a Backend object
func NewBackend(id string) *Backend {
	return &Backend{
		ID:   id,
		Type: TypeHTTP,
	}
}

// NewFrontend returns a ref to a Frontend object
func NewFrontend(id, bid string, route ...string) *Frontend {
	sort.StringSlice(route).Sort()
	rt := strings.Join(route, " && ")
	return &Frontend{
		ID:        id,
		BackendID: bid,
		Type:      TypeHTTP,
		Route:     rt,
	}
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

func (b Backend) Key() string { return fmt.Sprintf(bckndFmt, b.ID) }
func (s Server) Key() string {
	return fmt.Sprintf(srvrFmt, s.Backend, s.ID)
}
func (f Frontend) Key() string         { return fmt.Sprintf(frntndFmt, f.ID) }
func (f FrontendSettings) Key() string { return "" }
func (b BackendSettings) Key() string  { return "" }

func (b Backend) Val() (string, error)          { return encode(b) }
func (s Server) Val() (string, error)           { return encode(s) }
func (f Frontend) Val() (string, error)         { return encode(f) }
func (f FrontendSettings) Val() (string, error) { return "", nil }
func (b BackendSettings) Val() (string, error)  { return "", nil }

// DirKey returns the etcd directory key for this Backend
func (b Backend) DirKey() string { return fmt.Sprintf(bckndDirFmt, b.ID) }

// DirKey returns the etcd directory key for this Frontend
func (f Frontend) DirKey() string { return fmt.Sprintf(frntndDirFmt, f.ID) }

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

func (s Server) fields() map[string]interface{} {
	return map[string]interface{}{
		"server":  s.ID,
		"url":     s.URL.String(),
		"backend": s.Backend,
	}
}

func (f Frontend) fields() map[string]interface{} {
	return map[string]interface{}{
		"id":       f.ID,
		"backend":  f.BackendID,
		"type":     f.Type,
		"route":    f.Route,
		"settings": f.Settings.String(),
	}
}

func (b Backend) fields() map[string]interface{} {
	return map[string]interface{}{
		"id":       b.ID,
		"type":     b.Type,
		"settings": b.Settings.String(),
	}
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
	if e := json.NewEncoder(b).Encode(v); e != nil {
		return "", e
	}
	return strings.TrimSpace(HTMLUnescape(b.String())), nil
}

func decode(v VulcanObject, p []byte) error {
	return json.Unmarshal(p, v)
}

func buildRoute(ns string, a map[string]string) string {
	rt := []string{}
	if ns != "" {
		ns = fmt.Sprintf(".%s", ns)
	}
	for k, f := range rteConv {
		pk, ppk := fmt.Sprintf(annotationFmt, k, ns), fmt.Sprintf(annotationFmt, k, "")
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
