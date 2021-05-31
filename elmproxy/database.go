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
	Name    string `gorm:"index:pkgId,unique"`
	Version string `gorm:"index:pkgId,unique"`
	Hash    string
	Private bool
}

type PrivateNamespace struct {
	Name string `gorm:"primaryKey" json:"name"`
}

type Endpoint struct {
	Url  string `json:"url"`
	Hash string `json:"hash"`
}

type PackageManager interface {
	Initialize() error
	GetPackage(name, version string) (*Package, error)
	AddPackage(name, version string, private bool) (*Package, error)
	AddPackageFromString(pkg string) (*Package, error)
	GetAllPackages() ([]Package, error)
	GetPackagesSince(since uint64) ([]Package, error)
	BatchCreate([]Package) error
	// Get a count of public packages
	//
	GetPublicCount() (uint64, error)
	// Private packages
	GetPrivatePackageNamespaces() ([]PrivateNamespace, error)
	GetPrivatePackageNamespace(namespace string) (*PrivateNamespace, error)
	CreatePrivatePackageNamespace(name string) (*PrivateNamespace, error)
	UpdatePackage(*Package) (*Package, error)
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
	if err := db.AutoMigrate(&PrivateNamespace{}); err != nil {
		return err
	}
	m.db = db
	return nil
}

func (m *SqlitePackageManager) GetPackage(name, version string) (*Package, error) {
	pkg := &Package{}
	if err := m.db.First(pkg, "name = ? AND version = ?", name, version).Error; err != nil {
		return nil, err
	}
	return pkg, nil
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

func (m *SqlitePackageManager) GetPrivatePackageNamespaces() ([]PrivateNamespace, error) {
	var namespaces []PrivateNamespace
	if err := m.db.Find(&namespaces).Error; err != nil {
		return nil, err
	}
	return namespaces, nil
}

func (m *SqlitePackageManager) GetPrivatePackageNamespace(namespace string) (*PrivateNamespace, error) {
	ns := &PrivateNamespace{}
	if err := m.db.First(ns, "name = ?", namespace).Error; err != nil {
		return nil, err
	}
	return ns, nil
}

func (m *SqlitePackageManager) CreatePrivatePackageNamespace(namespace string) (*PrivateNamespace, error) {
	p := &PrivateNamespace{
		Name: namespace,
	}
	if err := m.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

func (m *SqlitePackageManager) UpdatePackage(pkg *Package) (*Package, error) {
	if err := m.db.Model(pkg).Updates(pkg).Error; err != nil {
		return nil, err
	}
	return pkg, nil
}
