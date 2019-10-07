package rest

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ipsusila/opt"
	"github.com/robfig/cron/v3"
)

type driverOptions struct {
	Format   string       `json:"format"`
	URI      string       `json:"uri"`
	CronSpec string       `json:"cronSpec"`
	Username string       `json:"username"`
	Password string       `json:"password"`
	Timeout  opt.Duration `json:"timeout"`
}

//restx driver configuration
type restDriver struct {
}

//restx connector
type restConnector struct {
	mu       sync.Mutex
	handler  func(f int) error
	op       driverOptions
	lastConf string
	c        *cron.Cron
}

// register restx driver
func init() {
	opt.Register("rest", &restDriver{})
}

// Connect to the configuration source. Connection must includes:
// - format			: string*
// - uri			: string*
// - cronSpec		: string*
// - username		: string*
// - password		: string*
func (dd *restDriver) Connect(h func(f int) error, prop *opt.Options) (opt.Connector, error) {
	op := driverOptions{
		Format:  opt.FormatJSON,
		Timeout: opt.Duration{Duration: 10 * time.Second},
	}
	if err := prop.AsStruct(&op); err != nil {
		return nil, err
	}

	rc := &restConnector{
		op:      op,
		handler: h,
	}
	if err := rc.connect(); err != nil {
		return nil, err
	}
	return rc, nil
}

func (rc *restConnector) connect() error {
	// try to connect
	_, err := rc.getConfig()
	if err != nil {
		return err
	}

	// monitor database change using cron
	if rc.handler != nil {
		rc.c.AddFunc(rc.op.CronSpec, func() {
			// get last loaded config
			rc.mu.Lock()
			lastConfig := rc.lastConf
			rc.mu.Unlock()

			// read config from REST server
			config, err := rc.getConfig()
			if err == nil && lastConfig != "" && config != lastConfig {
				if err := rc.handler(opt.SourceModified); err != nil {
					log.Printf("[OPT] restDriver cron, handler error: %v", err)
				}
			} else {
				log.Printf("[OPT] restDriver cron, get config error: %v", err)
			}
		})
	}
	return nil
}

func (rc *restConnector) getConfig() (string, error) {
	client := &http.Client{
		Timeout: rc.op.Timeout.Duration,
	}
	req, err := http.NewRequest("GET", rc.op.URI, nil)
	if err != nil {
		return "", err
	}
	req.Close = true
	if rc.op.Username != "" && rc.op.Password != "" {
		req.SetBasicAuth(rc.op.Username, rc.op.Password)
	}

	// execute request
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", err
	}
	content, err := ioutil.ReadAll(resp.Body)

	return string(content), err
}

// Load read configuration from restx. The database must be in JSON format
func (rc *restConnector) Load() (*opt.Options, error) {
	content, err := rc.getConfig()
	if err != nil {
		return nil, err
	}
	rc.mu.Lock()
	rc.lastConf = content
	rc.mu.Unlock()

	return opt.FromText(rc.lastConf, rc.op.Format)
}

// Store save configuration to restx
func (rc *restConnector) Store(v *opt.Options) error {
	client := &http.Client{
		Timeout: rc.op.Timeout.Duration,
	}
	rd := strings.NewReader(v.AsJSON())
	req, err := http.NewRequest("POST", rc.op.URI, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true
	if rc.op.Username != "" && rc.op.Password != "" {
		req.SetBasicAuth(rc.op.Username, rc.op.Password)
	}

	// execute request
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	return err
}

// Close restx connection
func (rc *restConnector) Close() error {
	if rc.c != nil {
		ctx := rc.c.Stop()
		<-ctx.Done()
	}

	return nil
}
