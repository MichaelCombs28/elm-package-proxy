package elmproxy

import (
	"fmt"
	"net/http"
	"strconv"

	"encoding/json"

	"github.com/gorilla/mux"

	log "github.com/sirupsen/logrus"

	"github.com/elazarl/goproxy"
)

// Routes

func ProxyHandler() func(r *http.Request) *http.Response {
	h := Router()
	return func(r *http.Request) *http.Response {
		w := NewWriterFacade()
		h.ServeHTTP(w, r)
		// Should use default proxy route if missing
		if w.statusCode == 404 {
			return nil
		}
		return w.ToResponse(r)
	}
}

func Router() http.Handler {
	mux := mux.NewRouter()
	mux.HandleFunc("/all-packages/since/{pkgNumber:[0-9]+}", packagesSince)
	mux.HandleFunc("/all-packages", allPackages)
	return mux
}

func allPackages(w http.ResponseWriter, r *http.Request) {
	rw.RLock()
	defer rw.RUnlock()
	p, err := Packages.GetAllPackages()
	if err != nil {
		log.Fatal(err)
	}
	m := make(map[string][]string)
	for _, pkg := range p {
		if versions, ok := m[pkg.Name]; ok {
			m[pkg.Name] = append(versions, pkg.Version)
		} else {
			m[pkg.Name] = []string{pkg.Version}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Fatal(err)
	}
	w.Write(b)
	w.WriteHeader(200)
}

func packagesSince(w http.ResponseWriter, r *http.Request) {
	rw.RLock()
	defer rw.RUnlock()
	since, _ := strconv.ParseInt(mux.Vars(r)["pkgNumber"], 10, 64)
	p, err := Packages.GetPackagesSince(uint64(since))
	if err != nil {
		log.Fatal(err)
	}
	out := make([]string, len(p))
	for i, pkg := range p {
		out[i] = fmt.Sprintf("%s/%s", pkg.Name, pkg.Version)
	}
	b, _ := json.Marshal(out)
	w.Write(b)
	w.WriteHeader(200)
}

// ResponseWriter Facade
//
type ResponseWriterFacade struct {
	headers    map[string][]string
	bytes      []byte
	statusCode int
	edited     bool
}

func NewWriterFacade() *ResponseWriterFacade {
	return &ResponseWriterFacade{
		headers:    make(map[string][]string),
		bytes:      nil,
		statusCode: 200,
		edited:     false,
	}
}

func (r *ResponseWriterFacade) Header() http.Header {
	r.edited = true
	return r.headers
}

func (r *ResponseWriterFacade) Write(b []byte) (int, error) {
	r.edited = true
	if r.bytes != nil {
		r.bytes = append(r.bytes, b...)
	} else {
		r.bytes = b
	}
	return len(b), nil
}

func (r *ResponseWriterFacade) WriteHeader(statusCode int) {
	r.edited = true
	r.statusCode = statusCode
}

func (w *ResponseWriterFacade) ToResponse(r *http.Request) *http.Response {
	if !w.edited {
		return nil
	}
	h := w.Header().Get("Content-Type")
	if h == "" {
		h = "text/plain"
	}
	return goproxy.NewResponse(r, h, w.statusCode, string(w.bytes))
}
