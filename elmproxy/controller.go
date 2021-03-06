package elmproxy

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"gorm.io/gorm"

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
	mux.UseEncodedPath()
	mux.HandleFunc("/all-packages/since/{pkgNumber:[0-9]+}", packagesSince)
	mux.HandleFunc("/all-packages", allPackages)
	mux.HandleFunc("/register", registerPackage)
	mux.HandleFunc("/packages/{group}/{name}/{version}/elm.json", elmJson)
	mux.HandleFunc("/packages/{group}/{name}/{version}/endpoint.json", endpoint)
	mux.HandleFunc("/private-package", privatePackageSubmit)
	return mux
}

func privatePackageSubmit(w http.ResponseWriter, r *http.Request) {
	/*
		// Download
		// Hash
		h := sha1.New()
		b, _ := ioutil.ReadAll(r.Body)
		h.Write(b)
		bs := h.Sum(nil)
	*/
}

type ElmJson struct {
	Private bool `json:"private"`
}

//gocyclo:ignore
func registerPackage(w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	*r2 = *r
	var b bytes.Buffer
	b.ReadFrom(r.Body)
	r.Body = ioutil.NopCloser(&b)
	r2.Body = ioutil.NopCloser(bytes.NewReader(b.Bytes()))
	f, _, _ := r2.FormFile("elm.json")
	dec := json.NewDecoder(f)
	ej := ElmJson{}
	dec.Decode(&ej)
	if !ej.Private {
		return
	}
	name := r.URL.Query().Get("name")
	version := r.URL.Query().Get("version")
	if _, err := Packages.GetPackage(name, version); err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Error(err.Error())
			http.Error(w, "Server Error.", 500)
			return
		}
	} else {
		http.Error(w, "Package has already been published.", 400)
		return
	}
	contentType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	defer r.Body.Close()
	if contentType != "multipart/form-data" {
		http.Error(w, "Content type must be multipart/form-data", 400)
		return
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	p, err := mr.NextPart()
	if err != nil {
		http.Error(w, "Invalid multipart payload", 400)
		return
	}
	path := filepath.Join(viper.GetString("services.storage.dir"), "packages", name, version)

	for err != io.EOF {
		switch p.FormName() {
		case "elm.json":
			var m map[string]interface{}
			if err := json.NewDecoder(p).Decode(&m); err != nil {
				http.Error(w, "Invalid elm.json", 400)
				return
			}
			n := m["name"].(string)
			//TODO Proper elm.json validation
			if n != name {
				http.Error(w, "Invalid elm.json", 400)
				return
			}
			if err := os.MkdirAll(path, 0777); err != nil {
				http.Error(w, "Server Error.", 500)
				return
			}
			f, err := os.OpenFile(filepath.Join(path, "elm.json"), os.O_CREATE|os.O_WRONLY, 0777)
			if err != nil {
				log.Error(err.Error())
				return
			}
			defer f.Close()
			enc := json.NewEncoder(f)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(m); err != nil {
				log.Error(err.Error())
			}
		case "docs.json":
			b, _ := ioutil.ReadAll(p)
			ioutil.WriteFile(filepath.Join(path, "docs.json"), b, 0777)
		case "README.md":
			b, _ := ioutil.ReadAll(p)
			ioutil.WriteFile(filepath.Join(path, "README.md"), b, 0777)
		case "github-hash":
			b, _ := ioutil.ReadAll(p)
			endpoint := Endpoint{
				Url:  getZipballUrl(name, version),
				Hash: string(b),
			}
			b, _ = json.Marshal(endpoint)
			ioutil.WriteFile(filepath.Join(path, "endpoint.json"), b, 0777)
		}
		p, err = mr.NextPart()
	}
	if _, err := Packages.AddPackage(name, version, true); err != nil {
		http.Error(w, "", 500)
	}

	w.Write([]byte(""))
	w.WriteHeader(201)
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
		out[i] = fmt.Sprintf("%s@%s", pkg.Name, pkg.Version)
	}
	b, _ := json.Marshal(out)
	w.Write(b)
	w.WriteHeader(200)
}

func elmJson(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := filepath.Join(viper.GetString("services.storage.dir"), "packages", vars["group"], vars["name"], vars["version"], "elm.json")
	if _, err := os.Stat(path); err != nil {
		return
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(404)
			return
		}
	}
	w.Write(b)
}

func endpoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := filepath.Join(viper.GetString("services.storage.dir"), "packages", vars["group"], vars["name"], vars["version"], "endpoint.json")

	hashFile := filepath.Join(path)
	b, err := ioutil.ReadFile(hashFile)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(404)
			return
		}
	}
	w.Write(b)
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
		h = "application/text"
	}
	return goproxy.NewResponse(r, h, w.statusCode, string(w.bytes))
}

func getZipballUrl(name, version string) string {
	return fmt.Sprintf("https://github.com/%s/zipball/%s/", name, version)
}

func fetchExternalZipball(name, version string) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://github.com/%s/zipball/%s/", name, version)
	req, _ := http.NewRequest("GET", url, nil)
	token := viper.GetString("credentials.github")
	if token != "" {
		req.Header.Add("Authorization", "token "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
