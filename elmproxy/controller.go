package elmproxy

import (
	"fmt"
	"net/http"
	"os"

	"encoding/json"

	"github.com/gorilla/mux"

	log "github.com/sirupsen/logrus"

	"github.com/elazarl/goproxy"
)

type ResponseWriterFacade struct {
	headers    map[string][]string
	bytes      []byte
	statusCode int
	edited     bool
}

var _ http.ResponseWriter = &ResponseWriterFacade{}

func Initialize() func(r *http.Request) *http.Response {
	// Create storage dir if not exists
	_, err := os.Stat(*storageDirectory)
	if os.IsNotExist(err) {
		err := os.Mkdir(*storageDirectory, 0777)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else if err != nil {
		log.Fatal(err.Error())
	}

	if _, err := os.Stat(fmt.Sprintf("%s/packages/", *storageDirectory)); err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(fmt.Sprintf("%s/packages/", *storageDirectory), 0777); err != nil {
				log.Fatal(err.Error())
			}
		} else {
			log.Fatal(err)
		}
	}

	mux := mux.NewRouter()
	mux.HandleFunc("/all-packages/since/{pkgNumber:[0-9]+}", packagesSince)
	mux.HandleFunc("/all-packages", allPackages)

	return func(r *http.Request) *http.Response {
		w := NewWriterFacade()
		mux.ServeHTTP(w, r)
		// Should use default proxy route if missing
		if w.statusCode == 404 {
			return nil
		}
		return w.ToResponse(r)
	}
}

func allPackages(w http.ResponseWriter, r *http.Request) {
	p, err := FDataStore.GetAllPackages()
	if err != nil {
		log.Fatal(err)
	}

	if p == nil {
		return
	}

	b, err := json.Marshal(p.Packages())
	if err != nil {
		log.Fatal(err)
	}
	w.Write(b)
}

func packagesSince(w http.ResponseWriter, r *http.Request) {
	since := mux.Vars(r)["pkgNumber"]
	log.Debug("Since: ", since)
	p := []string{
		"elm/core@1.0.0",
	}
	b, _ := json.Marshal(p)
	w.Write(b)
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
