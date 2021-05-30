package elmproxy

import (
	"errors"
	"flag"
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	packageDirectory *string = flag.String("package-dir", "./data/packages/", "Where private packages are stored")
	dbName           *string = flag.String("db-name", "", "Database name")
	rw               sync.RWMutex
	Packages         PackageManager = &SqlitePackageManager{}
)

func Initialize() error {
	if err := Packages.Initialize(); err != nil {
		return err
	}
	if _, err := os.Stat(*packageDirectory); err != nil {
		if os.IsNotExist(err) {
			log.Debugf("Creating package directory at %s", *packageDirectory)
			if err := os.MkdirAll(*packageDirectory, 0777); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return fetchPackages()
}

type Package struct {
	gorm.Model
	Name    string
	Version string
	Private bool
}

type PackageManager interface {
	Initialize() error
	AddPackage(name, version string, private bool) (*Package, error)
	AddPackageFromString(pkg string) (*Package, error)
	GetAllPackages() ([]Package, error)
	GetPackagesSince(since uint64) ([]Package, error)
	BatchCreate([]Package) error
	// Get a count of public packages
	//
	GetPublicCount() (uint64, error)
}

type SqlitePackageManager struct {
	db *gorm.DB
}

func (m *SqlitePackageManager) BatchCreate(pkgs []Package) error {
	return m.db.CreateInBatches(pkgs, 100).Error
}

func (m *SqlitePackageManager) Initialize() error {
	if *dbName == "" {
		*dbName = "db.sqlite3"
	}
	db, err := gorm.Open(sqlite.Open(*dbName))
	if err != nil {
		return err
	}
	if err := db.AutoMigrate(&Package{}); err != nil {
		return err
	}
	m.db = db
	return nil
}

func (m *SqlitePackageManager) AddPackage(name, version string, private bool) (*Package, error) {
	p := &Package{
		Name:    name,
		Version: version,
		Private: private,
	}
	if err := m.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (m *SqlitePackageManager) AddPackageFromString(pkg string) (p *Package, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("Invalid string.")
			return
		}
	}()

	splt := strings.Split(pkg, "@")
	p = &Package{
		Name:    splt[0],
		Version: splt[1],
	}
	if err := m.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (m *SqlitePackageManager) GetAllPackages() ([]Package, error) {
	var packages []Package
	if err := m.db.Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

func (m *SqlitePackageManager) GetPackagesSince(since uint64) ([]Package, error) {
	var packages []Package
	if err := m.db.Model(&Package{}).Where("ID > ?", since).Find(&packages).Error; err != nil {
		return nil, err
	}
	return packages, nil
}

func (m *SqlitePackageManager) GetPublicCount() (uint64, error) {
	var i int64
	if err := m.db.Model(&Package{}).Where("Private = ?", false).Count(&i).Error; err != nil {
		log.Error("Error get count ", err)
		return 0, err
	}
	return uint64(i), nil
}
