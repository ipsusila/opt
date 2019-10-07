package opt

import (
	"errors"
	"sync"
)

type configurableItem struct {
	section string
	conf    Configurable
}

// Configurable represent an object/application that is configurable
type Configurable interface {
	Configure(op *Options, first bool)
}

// Configurator stores application wide configuration
type Configurator struct {
	mu      sync.RWMutex
	conn    Connector
	lastCfg *Options
	items   []configurableItem
}

// NewConfigurator open configuration with given driver and property
func NewConfigurator(driver string, prop *Options) (*Configurator, error) {
	// load the driver
	drv := DriverFor(driver)
	if drv == nil {
		return nil, errors.New("can not find configuration driver " + driver + ", forget to import?")
	}

	// create configurator
	cfg := Configurator{}
	conn, err := drv.Connect(cfg.sourceChanged, prop)
	if err != nil {
		return nil, err
	}
	cfg.lastCfg, err = conn.Load()
	if err != nil {
		conn.Close()
		return nil, err
	}
	cfg.conn = conn

	return &cfg, nil
}

func (cfg *Configurator) sourceChanged(f int) error {
	// if new configuration is available, load it.
	// if the configuration is changed, reconfigure all registered configurable
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	newCfg, err := cfg.conn.Load()
	if err != nil {
		return err
	}
	if newCfg == nil {
		return errors.New("loaded configuration return <nil>")
	}

	if !newCfg.EqualTo(cfg.lastCfg) {
		cfg.lastCfg = newCfg
		for _, item := range cfg.items {
			item.conf.Configure(newCfg.Get(item.section), false)
		}
	}

	return nil
}

// Valid returns true if connector is set and configuration loaded
func (cfg *Configurator) Valid() bool {
	return cfg.conn != nil && cfg.lastCfg != nil
}

// Get return configuration for given key
func (cfg *Configurator) Get(key string) *Options {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	if cfg.lastCfg != nil {
		return cfg.lastCfg.Get(key)
	}
	return New()
}

// Store save configuration
func (cfg *Configurator) Store() error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	if cfg.Valid() {
		return cfg.conn.Store(cfg.lastCfg)
	}
	return nil
}

// Load configuration from underlying source and
// if configure set to true, configure all registered objects
func (cfg *Configurator) Load(configure bool) error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	if cfg.conn != nil {
		newCfg, err := cfg.conn.Load()
		if err != nil {
			return err
		}
		cfg.lastCfg = newCfg
		if configure {
			cfg.Configure()
		}
	}
	return nil
}

// Configure all registered configurable
func (cfg *Configurator) Configure() {
	if cfg.lastCfg != nil {
		for _, item := range cfg.items {
			item.conf.Configure(cfg.lastCfg.Get(item.section), false)
		}
	}
}

// Register configurable to be managed by this configurator
func (cfg *Configurator) Register(section string, c Configurable) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	// duplicate check
	for _, v := range cfg.items {
		if v.conf == c {
			return
		}
	}

	// append new client then reconfigure
	cfg.items = append(cfg.items, configurableItem{section: section, conf: c})
	if cfg.lastCfg != nil {
		c.Configure(cfg.lastCfg, true)
	}
}

// Close configurator
func (cfg *Configurator) Close() error {
	if cfg.conn != nil {
		c := cfg.conn
		cfg.conn = nil
		return c.Close()
	}
	return nil
}
