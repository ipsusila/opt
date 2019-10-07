package db

import (
	"errors"
	"log"
	"strings"
	"sync"

	"github.com/ipsusila/opt"
	"github.com/jmoiron/sqlx"
	"github.com/robfig/cron/v3"
)

type driverOptions struct {
	Driver     string `json:"driver"`
	LoadQuery  string `json:"loadQuery"`
	StoreQuery string `json:"storeQuery"`
	DSN        string `json:"dsn"`
	CronSpec   string `json:"cronSpec"`
}

//dbx driver configuration
type dbDriver struct {
}

//dbx connector
type dbConnector struct {
	mu       sync.Mutex
	handler  func(f int) error
	op       driverOptions
	lastConf string
	c        *cron.Cron
	dbx      *sqlx.DB
}

// register dbx driver
func init() {
	opt.Register("database", &dbDriver{})
}

// Connect to the configuration source. Connection must includes:
// - driver			: string*
// - selectQuery	: string*
// - storeQuery		: string*
// - dsn			: string*
// - cronSpec		: string*
func (dd *dbDriver) Connect(h func(f int) error, prop *opt.Options) (opt.Connector, error) {
	op := driverOptions{
		Driver: "sqlite3",
	}
	if err := prop.AsStruct(&op); err != nil {
		return nil, err
	}
	dc := &dbConnector{
		op:      op,
		handler: h,
		c:       cron.New(),
		dbx:     nil,
	}
	if err := dc.connect(); err != nil {
		return nil, err
	}
	return dc, nil
}

// connect to database
func (dc *dbConnector) connect() error {
	dbx, err := sqlx.Open(dc.op.Driver, dc.op.DSN)
	if err != nil {
		return err
	}
	if err := dbx.Ping(); err != nil {
		return err
	}
	dc.dbx = dbx

	// monitor database change using cron
	if dc.handler != nil {
		dc.c.AddFunc(dc.op.CronSpec, func() {
			// get last loaded config
			dc.mu.Lock()
			lastConfig := dc.lastConf
			dc.mu.Unlock()

			// read config in DB
			config := ""
			if err := dbx.Get(&config, dc.op.LoadQuery); err == nil {
				if lastConfig != "" && config != lastConfig {
					if err := dc.handler(opt.SourceModified); err != nil {
						log.Printf("[OPT] dbDriver cron, handler error: %v", err)
					}
				}
			} else {
				log.Printf("[OPT] dbDriver cron, get config error: %v", err)
			}
		})
	}

	return nil
}

// Load read configuration from dbx. The database must be in JSON format
func (dc *dbConnector) Load() (*opt.Options, error) {
	// load configuration from dbx
	if dc.dbx == nil {
		return nil, errors.New("database not connected")
	}
	config := ""
	if err := dc.dbx.Get(&config, dc.op.LoadQuery); err != nil {
		return nil, err
	}
	op, err := opt.FromReader(strings.NewReader(config), opt.FormatJSON)
	if err != nil {
		return nil, err
	}
	dc.mu.Lock()
	dc.lastConf = config
	dc.mu.Unlock()

	return op, nil
}

// Store save configuration to dbx
func (dc *dbConnector) Store(v *opt.Options) error {
	if v == nil {
		return errors.New("dbConnector: config parameter is nil")
	}
	if dc.dbx == nil {
		return errors.New("database not connected")
	}

	// execute query
	_, err := dc.dbx.Exec(dc.op.StoreQuery, v.AsJSON())
	return err
}

// Close dbx connection
func (dc *dbConnector) Close() error {
	if dc.c != nil {
		ctx := dc.c.Stop()
		<-ctx.Done()
	}
	if dc.dbx != nil {
		err := dc.dbx.Close()
		dc.dbx = nil

		return err
	}
	return nil
}
