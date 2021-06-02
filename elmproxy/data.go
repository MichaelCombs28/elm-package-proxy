package elmproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Continually synchronizes data with package.elm-lang.org
//
var (
	httpClient *http.Client = &http.Client{
		Timeout: time.Second * 20,
	}
	lastSync int64 = 0
)

func SyncWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Second * time.Duration(viper.GetInt64("services.sync.interval")))
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Will skip if recently forced to sync
			log.Debug("SyncWorker tick, fetching packages.")
			err := fetchPackages()
			if err != nil {
				log.Error(err)
				continue
			}
		}
	}
	log.Debug("SyncWorker is done.")
}

func fetchPackages() error {
	rw.Lock()
	defer rw.Unlock()
	if lastSync+viper.GetInt64("services.sync.interval")/2 < time.Now().Unix() {
		//Fetch packages

		since, err := Packages.GetPublicCount()
		if err != nil {
			return err
		}

		if since == 0 {
			log.Debug("Initializing database for the first time.")
			req, _ := http.NewRequest("GET", "https://package.elm-lang.org/all-packages", nil)
			resp, err := httpClient.Do(req)
			if err != nil {
				return err
			}
			var m map[string][]string
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&m); err != nil {
				return err
			}
			p := 0
			total := int(packageVersionSum(m))
			log.Debugf("Received %d total packages.", total)
			pkgs := make([]Package, total)
			for name, versions := range m {
				for _, version := range versions {
					pkgs[p] = Package{
						Name:    name,
						Version: version,
					}
					p += 1
				}
			}
			if err := Packages.BatchCreate(pkgs); err != nil {
				return err
			}
			lastSync = time.Now().Unix()
			return nil
		}

		url := fmt.Sprintf("https://package.elm-lang.org/all-packages/since/%d", since)
		log.Debug("Fetching packages with url: ", url)
		req, _ := http.NewRequest("GET", fmt.Sprintf("https://package.elm-lang.org/all-packages/since/%d", since), nil)
		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		dec := json.NewDecoder(resp.Body)
		var versions []string
		if err := dec.Decode(&versions); err != nil {
			return err
		}
		log.Debugf("Received %d new package(s)", len(versions))
		for _, v := range versions {
			if _, err := Packages.AddPackageFromString(v); err != nil {
				return err
			}
		}
		lastSync = time.Now().Unix()
	}
	return nil
}
func packageVersionSum(p map[string][]string) int64 {
	var i int64
	for _, v := range p {
		i += int64(len(v))
	}
	return i
}

/*
import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	FDataStore       DataStore    = &fileDataStore{}
	storageDirectory *string      = flag.String("storage-dir", "./data", "Storage directory for server.")
	httpClient       *http.Client = &http.Client{
		Timeout: time.Second * 20,
	}
)

// System packages
//
type Packages struct {
	InitializedCount `json:"initializedCount"`
	OfficialPackageCount `json:"officialPackageCount"`
	OfficialPackages PackageList `json:"officialPackages"`
	ManagedPackages  PackageList `json:"managedPackages"`
	ManagedPackageCount
	// For use with since
	PackageQueue     []string    `json:"packageQueue"`
}

func (p *Packages) Packages() PackageList {
	o := make(PackageList)
	for k, v := range p.OfficialPackages {
		o[k] = v
	}
	for k, v := range p.ManagedPackages {
		o[k] = v
	}
	return o
}

// List of package entries
//
type PackageList = map[string][]string

// Package endpoint with download & hash
//
type PackageEndpoint struct {
	Url  string `json:"url"`
	Hash string `json:"hash"`
}

// Datastore interface used to store and retrieve elm packages.
//
type DataStore interface {
	// Retrieve all packages from system cache, fetch packages
	// if cache is empty, or does not exist.
	//
	GetAllPackages() (*Packages, error)
	GetPackagesSince(since int64) ([]string, error)
}

type fileDataStore struct {
}

// Gets all system packages, or retrieves them if
// system cache is empty
//
func (f *fileDataStore) GetAllPackages() (*Packages, error) {
	b, err := ioutil.ReadFile(packagesFile())
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug("System packages.json does not exist, creating.")
			log.Debug("Fetching package list from package.elm-lang.org")
			b, err = fetchAllPackages()
			if err != nil {
				return nil, err
			}
			var pl PackageList
			err := json.Unmarshal(b, &pl)
			if err != nil {
				return nil, err
			}
			p := &Packages{}
			p.LastSync = time.Now().Unix()
			p.OfficialPackages = pl
			p.OfficialPackageCount = packageVersionSum(pl)
			p.ManagedPackageCount = 0
			p.ManagedPackages = make(PackageList)

			j, err := json.Marshal(p)
			if err != nil {
				return nil, err
			}
			log.Debug("Creating package cache.")
			err = ioutil.WriteFile(packagesFile(), j, 0777)
			if err != nil {
				return nil, err
			}
			return p, nil
		} else {
			return nil, err
		}
	}
	p := &Packages{}
	err = json.Unmarshal(b, p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (f *fileDataStore) GetPackagesSince(since int64) ([]string, error) {
	p, err := f.GetAllPackages()
	if err != nil {
		return nil, err
	}

	officialCount := since - packageVersionSum(p.OfficialPackages
	managedSince := since - p.OfficialPackageCount
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://package.elm-lang.org/all-packages/since/%d", officialSince), nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	var diffs []string
	err = dec.Decode(&diffs)
	if err != nil {
		return nil, err
	}
	for _, diff := range diffs {
		splt := strings.Split(diff, "@")
		pkg, ok := p.ManagedPackages[splt[0]]
		if !ok {
			p.ManagedPackages[splt[0]] = []string{splt[1]}
		} else {
			p.ManagedPackages[splt[0]] = append(pkg, splt[1])
		}
	}

}

func (f *fileDataStore) GetManagedPackages() (*Packages, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("%s/packages.json", *storageDirectory))
	if err != nil {
		if os.IsNotExist(err) {
			// No Managed packages, should go all the way to package.elm-lang.org
			return nil, nil
		} else {
			// Real error, likely permissions
			return nil, err
		}
	}

	p := &Packages{}
	err = json.Unmarshal(b, p)
	if err != nil {
		//File is likely invalid
		log.Error("packages.json is invalid.")
		return nil, err
	}
	req, _ := http.NewRequest("GET", "https://package.elm-lang.org/all-packages", nil)
	response, err := httpClient.Do(req)
	if err != nil {
		log.Fatal("Error making request to package.elm-lang.org.", err.Error())
	}
	b, err = ioutil.ReadAll(response.Body)
	if err != nil {
		// Network has dropped or server has closed,
		// maybe bad request somehow?
		log.Fatal(err.Error())
	}
	// Response map from package.elm-lang.org
	lst := make(map[string][]string)
	err = json.Unmarshal(b, lst)
	if err != nil {
		// Server likely returned an invalid response
		log.Fatal(err.Error())
	}

	for k, v := range p.OfficialPackages {
		lst[k] = v
	}
	p.OfficialPackages = lst
	return p, nil
}

func (f *fileDataStore) GetPackageEndpoint(packageName, version string) (*PackageEndpoint, error) {
	// TODO Striping
	path := fmt.Sprintf("%s/packages/%s@%s/endpoint.json", *storageDirectory, packageName, version)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Not managed by this server
			return nil, nil
		}
		log.Fatal(err.Error())
	}

	p := &PackageEndpoint{}
	err = json.Unmarshal(b, p)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Likely data corruption in %s", path))
	}

	return p, nil
}

func (f *fileDataStore) GetPackageElmJson(packageName, version string) ([]byte, error) {
	path := fmt.Sprintf("%s/packages/%s@%s/elm.json", *storageDirectory, packageName, version)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Not managed by this server
			return nil, nil
		}
		log.Fatal(err.Error())
	}
	return b, nil
}

// HELPERS

func packagesFile() string {
	return fmt.Sprintf("%s/packages.json", *storageDirectory)
}

func fetchAllPackages() ([]byte, error) {
	req, _ := http.NewRequest("GET", "https://package.elm-lang.org/all-packages", nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("Received invalid response from package.elm-lang.org")
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}
*/
