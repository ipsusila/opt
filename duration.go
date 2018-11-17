package opt

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
)

//Duration stores duration for json decode/encode
type Duration struct {
	time.Duration
}

//MarshalJSON converts duration to string
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

//UnmarshalJSON convert duration stream to time.Duration
func (d *Duration) UnmarshalJSON(data []byte) error {
	var err error
	var v interface{}
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
	case string:
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	default:
		return errors.Errorf("invalid duration: `%s`", string(data))
	}

	return nil
}
