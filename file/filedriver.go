package file

import (
	"errors"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ipsusila/opt"
)

type driverOptions struct {
	Format         string       `json:"format"`
	FileName       string       `json:"fileName"`
	EventDelay     opt.Duration `json:"eventDelay"`
	EventQueueSize int          `json:"eventQueueSize"`
}

//file driver configuration
type fileDriver struct {
}

//file connector
type fileConnector struct {
	handler func(f int) error
	op      driverOptions
	quit    chan bool
	done    chan bool
	watcher *fsnotify.Watcher
}

// register file driver
func init() {
	opt.Register("file", &fileDriver{})
}

// Connect to the configuration source. Connection options:
// - format			: string
// - fileName		: string*
// - eventDelay		: duration
// - eventQueueSize	: int
func (fd *fileDriver) Connect(h func(f int) error, prop *opt.Options) (opt.Connector, error) {
	op := driverOptions{
		Format:         opt.FormatAuto,
		EventDelay:     opt.Duration{Duration: 2 * time.Second},
		EventQueueSize: 10,
	}
	if err := prop.AsStruct(&op); err != nil {
		return nil, err
	}

	// create file connector
	fc := &fileConnector{
		op:      op,
		handler: h,
		quit:    make(chan bool, 1),
		done:    make(chan bool, 1),
	}
	close(fc.done)
	if err := fc.openFile(); err != nil {
		return nil, err
	}

	return fc, nil
}

func (fc *fileConnector) watchFileChanged() {
	defer close(fc.done)

	// create ticker, every 2 seconds
	ticker := time.NewTicker(fc.op.EventDelay.Duration)
	defer ticker.Stop()

	// Fetch latest event only
	changes := make(chan fsnotify.Event, fc.op.EventQueueSize)
	fnLastEvent := func() (fsnotify.Event, bool) {
		found := false
		ev := fsnotify.Event{}
		for {
			select {
			case e, ok := <-changes:
				if ok {
					ev = e
					found = true
				}
			default:
				return ev, found
			}
		}
		// need this?
		// return ev, found
	}

	for {
		select {
		case <-fc.quit:
			return
		case <-ticker.C:
			if ev, found := fnLastEvent(); found {
				// inform handler that configuration source changed
				if err := fc.handler(int(ev.Op)); err != nil {
					//TODO: display error
				}
			}
		case event, ok := <-fc.watcher.Events:
			if !ok {
				return
			}
			changes <- event
		case err, ok := <-fc.watcher.Errors:
			if !ok {
				return
			}
			if err != nil {
				// TODO: display error
			}
		}
	}
}

// open the file
func (fc *fileConnector) openFile() error {
	// verify format
	if fc.op.Format != opt.FormatAuto && fc.op.Format != opt.FormatHJSON && fc.op.Format != opt.FormatJSON {
		return errors.New("unsupported format " + fc.op.Format)
	}

	// try to open file
	fd, err := os.Open(fc.op.FileName)
	if err != nil {
		return err
	}
	defer fd.Close()

	// watch configuration
	fc.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// if handler not defined, do not watch the file
	if fc.handler == nil {
		return nil
	}

	// start watching loop
	fc.done = make(chan bool, 1)
	go fc.watchFileChanged()

	// watch config file
	return fc.watcher.Add(fc.op.FileName)
}

// Load read configuration from file
func (fc *fileConnector) Load() (*opt.Options, error) {
	// load configuration from file
	return opt.FromFile(fc.op.FileName, fc.op.Format)
}

// Store save configuration to file
func (fc *fileConnector) Store(v *opt.Options) error {
	if v == nil {
		return errors.New("fileConnector: config parameter is nil")
	}
	return opt.ToFile(v, fc.op.FileName, fc.op.Format)
}

// Close file connection
func (fc *fileConnector) Close() error {
	close(fc.quit)
	<-fc.done
	if fc.watcher != nil {
		fc.watcher.Close()
		fc.watcher = nil
	}
	return nil
}
