package opt

import (
	"sort"
	"sync"
)

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]Driver)
)

// Marks source changed event
const (
	SourceRemoved = iota
	SourceModified
)

// SourceChangedFunc defines handler for source change
type SourceChangedFunc func(f int) error

// Driver is reponsible for loading/saving configuration
type Driver interface {
	Connect(h func(f int) error, prop *Options) (Connector, error)
}

// Connector connects to configuration source
type Connector interface {
	Load() (*Options, error)
	Store(v *Options) error
	Close() error
}

// Register makes a driver available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.
func Register(name string, driver Driver) {
	driversMu.Lock()
	defer driversMu.Unlock()
	if driver == nil {
		panic("alert: Register driver is nil")
	}
	if _, dup := drivers[name]; dup {
		panic("alert: Register called twice for driver " + name)
	}
	drivers[name] = driver
}

func unregisterAllDrivers() {
	driversMu.Lock()
	defer driversMu.Unlock()

	// For tests.
	drivers = make(map[string]Driver)
}

// Drivers returns a sorted list of the names of the registered drivers.
func Drivers() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	var list []string
	for name := range drivers {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

// DriverFor return Driver for given name
func DriverFor(name string) Driver {
	driversMu.RLock()
	defer driversMu.RUnlock()

	if driver, ok := drivers[name]; ok {
		return driver
	}

	return nil
}
