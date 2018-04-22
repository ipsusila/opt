package opt

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"sync"

	hjson "github.com/hjson/hjson-go"
	"github.com/pkg/errors"
)

// ------------------------------------------------------------------------------------------------

const (
	delimField = '='
	delimOpt   = ';'
	maxKVLen   = 1024 * 1024
	maxLen     = 1024 * 1024 * 100
)

//Configuration format
const (
	FormatAuto  = ""
	FormatHJSON = "hjson" //Human json
	FormatJSON  = "json"
)

// replace any escape character
var rplEscape = strings.NewReplacer("\\n", "\n", "\\r", "\r", "\\\\", "\\", "\\t", "\t")

// line escape
var rplLineEscape = strings.NewReplacer("\\\r\n", "", "\\\r", "", "\\\n", "")

// replace ; and = to escaped format
var rplDelimiter = strings.NewReplacer(";", "\\;", "=", "\\=")

//Options store device option parameters
type Options struct {
	sync.RWMutex
	filePath string
	options  map[string]interface{}
}

//New create option structure
func New() *Options {
	opt := &Options{
		options: make(map[string]interface{}),
	}

	return opt
}

//FromReader create options from IO reader
func FromReader(reader io.Reader, format string) (*Options, error) {
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read options")
	}
	var data map[string]interface{}

	switch strings.ToLower(format) {
	case FormatHJSON:
		if err := hjson.Unmarshal(content, &data); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal hjson")
		}
		o := &Options{}
		o.Assign(data)

		return o, nil
	case FormatJSON:
		if err := json.Unmarshal(content, &data); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal json")
		}
		o := &Options{}
		o.Assign(data)
	}

	return nil, errors.Errorf("Not supported options format %s", format)
}

//FromText create options from text
func FromText(cfgText string, format string) (*Options, error) {
	reader := strings.NewReader(cfgText)
	return FromReader(reader, format)
}

//FromFile read options from given file
func FromFile(filePath string, format string) (*Options, error) {
	ext := ""
	if len(format) == 0 {
		ext = strings.Trim(path.Ext(filePath), ".")
	} else {
		ext = format
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", filePath)
	}
	defer f.Close()

	o, err := FromReader(f, ext)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse stream %s", filePath)
	}

	//save file path
	o.filePath = filePath

	return o, nil
}

func (o *Options) newContainer(key string) (map[string]interface{}, string) {
	keyItems := strings.Split(key, ".")
	nItems := len(keyItems)
	if nItems <= 1 {
		return o.options, key
	}

	ne := nItems - 1
	mapItem := o.options
	for k := 0; k < ne; k++ {
		key = keyItems[k]
		val, ok := mapItem[key]
		if !ok {
			newContainer := make(map[string]interface{})
			mapItem[key] = newContainer
			mapItem = newContainer
		} else {
			mapItem, ok = val.(map[string]interface{})
			if !ok {
				newContainer := make(map[string]interface{})
				mapItem[key] = newContainer
				mapItem = newContainer
			}
		}
	}

	return mapItem, keyItems[ne]
}

func (o *Options) getContainer(key string) (map[string]interface{}, string) {
	keyItems := strings.Split(key, ".")
	nItems := len(keyItems)
	if nItems <= 1 {
		return o.options, key
	}

	ne := nItems - 1
	mapEmpty := make(map[string]interface{})
	mapItem := o.options
	for k := 0; k < ne; k++ {
		key = keyItems[k]
		val, ok := mapItem[key]
		if !ok {
			return mapEmpty, keyItems[k+1]
		}

		mapItem, ok = val.(map[string]interface{})
		if !ok {
			return mapEmpty, keyItems[k+1]
		}
	}

	return mapItem, keyItems[ne]
}

func (o *Options) getPath(name string) string {
	//name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "@") {
		name = strings.Replace(name, "@", "", 1)
	}
	return path.Join(path.Dir(o.filePath), name)
}

//interface value to text
func (o *Options) asText(val interface{}) string {
	//first use type assertion
	strVal := ""
	if optVal, ok := val.(string); ok {
		strVal = rplEscape.Replace(optVal)
	} else {
		strVal = fmt.Sprintf("%v", val)
	}

	return strVal
}

//special characters: \ = and ;
func (o *Options) escape(val interface{}) string {
	//Do not escape for non string
	txt, ok := val.(string)
	if !ok {
		return fmt.Sprintf("%v", val)
	}

	//Escape string if needed
	n := len(txt)
	if n > maxKVLen {
		msg := fmt.Sprintf("Key/Value len is greater than allowed limit (%d > %d)", n, maxKVLen)
		panic(msg)
	}

	rbuf := make([]rune, 2*n)
	rp := 0

	//special characters
	ESCAPE := '\\'
	for _, ch := range txt {
		switch ch {
		case '\r':
			rbuf[rp] = ESCAPE
			rp++
			rbuf[rp] = 'r'
			rp++
		case '\n':
			rbuf[rp] = ESCAPE
			rp++
			rbuf[rp] = 'n'
			rp++
		case '\t':
			rbuf[rp] = ESCAPE
			rp++
			rbuf[rp] = 't'
			rp++
		case ESCAPE, delimField, delimOpt:
			rbuf[rp] = ESCAPE
			rp++
			rbuf[rp] = ch
			rp++
		default:
			rbuf[rp] = ch
			rp++
		}
	}

	return string(rbuf[:rp])
}

//String converts options to string
func (o *Options) String() string {
	o.RLock()
	defer o.RUnlock()

	//convert to string
	format := func(obj interface{}) string {
		if str, ok := obj.(string); ok {
			return rplDelimiter.Replace(str)
		}
		return fmt.Sprintf("%v", obj)
	}

	//format as key=value;opt=value;
	text := ""
	fDelim := string(delimField)
	oDelim := string(delimOpt)
	for key, val := range o.options {
		vMap, isMap := val.(map[string]interface{})
		valStr := ""
		if isMap {
			op := &Options{options: vMap}
			valStr = "{" + op.String() + "}"
		} else {
			valStr = format(val)
		}
		text += key + fDelim + valStr + oDelim
	}
	return text
}

//Format options for display
func (o *Options) Format(optDelim string) string {
	o.RLock()
	defer o.RUnlock()

	text := ""
	fDelim := string(delimField)
	for key, val := range o.options {
		vMap, isMap := val.(map[string]interface{})
		valStr := ""
		if isMap {
			op := &Options{options: vMap}
			valStr = "{" + op.Format(optDelim) + "}"
		} else {
			valStr = o.asText(val)
		}
		text += key + fDelim + valStr + optDelim
	}
	return text
}

//AsStruct method convert configuration contents to struct.
//Process: options -> JSON -> struct
func (o *Options) AsStruct(out interface{}) error {
	stream, err := json.Marshal(o.options)
	if err != nil {
		return err
	}

	//convert to struct
	err = json.Unmarshal(stream, out)
	if err != nil {
		return err
	}
	return nil
}

//AsJSON dump content as JSON
func (o *Options) AsJSON() string {
	stream, err := json.MarshalIndent(o.options, "", "  ")
	if err != nil {
		return "<ERROR>:" + err.Error()
	}
	return string(stream)
}

//GetStringArray returns array of string
func (o *Options) GetStringArray(key string) []string {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return nil
	}

	va, ok := val.([]interface{})
	if !ok {
		return nil
	}

	res := []string{}
	for _, val := range va {
		switch v := val.(type) {
		case string:
			res = append(res, v)
		case fmt.Stringer:
			res = append(res, v.String())
		default:
			//convert to string with fmt.Sprintf
			res = append(res, fmt.Sprint(val))
		}
	}

	return res
}

//GetString retrieves option value. If not exist, return default value
func (o *Options) GetString(key, def string) string {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return def
	}

	return o.asText(val)
}

func (o *Options) GetDuration(key string, def time.Duration) time.Duration {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return def
	}

	switch v := val.(type) {
	case float64:
		return time.Duration(v)
	case float32:
		return time.Duration(v)
	case int64:
		return time.Duration(v)
	case uint64:
		return time.Duration(v)
	case int32:
		return time.Duration(v)
	case uint32:
		return time.Duration(v)
	case int:
		return time.Duration(v)
	case int16:
		return time.Duration(v)
	case uint16:
		return time.Duration(v)
	case int8:
		return time.Duration(v)
	case uint8:
		return time.Duration(v)
	default:
		//convert to string first then, convert to duration
		dur, err := time.ParseDuration(o.asText(val))
		if err != nil {
			return def
		}
		return dur
	}
}

//GetObjectArray returns options in object form
func (o *Options) GetObjectArray(key string) []*Options {
	/*
		//assume the value is an object
		vMap, ok := val.(map[string]interface{})
		if !ok {
			return New()
		}

		newOpt := &Options{options: vMap}
		return newOpt
	*/

	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return nil
	}

	va, ok := val.([]interface{})
	if !ok {
		return nil
	}

	res := []*Options{}
	for _, val := range va {
		if v, ok := val.(map[string]interface{}); ok {
			newOpt := &Options{
				options: v,
			}
			res = append(res, newOpt)
		}
	}

	return res
}

func (o *Options) GetInt64Array(key string) []int64 {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return nil
	}

	va, ok := val.([]interface{})
	if !ok {
		return nil
	}

	res := []int64{}
	for _, val := range va {
		switch v := val.(type) {
		case float64:
			res = append(res, int64(v))
		case float32:
			res = append(res, int64(v))
		case int64:
			res = append(res, v)
		case uint64:
			res = append(res, int64(v))
		case int32:
			res = append(res, int64(v))
		case uint32:
			res = append(res, int64(v))
		case int:
			res = append(res, int64(v))
		case int16:
			res = append(res, int64(v))
		case uint16:
			res = append(res, int64(v))
		case int8:
			res = append(res, int64(v))
		case uint8:
			res = append(res, int64(v))
		default:
			//convert to string first then, convert to integer
			vi, err := strconv.ParseInt(o.asText(val), 10, 64)
			if err == nil {
				res = append(res, int64(vi))
			}
		}
	}

	return res
}

func (o *Options) GetFloat64Array(key string) []float64 {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return nil
	}

	va, ok := val.([]interface{})
	if !ok {
		return nil
	}

	res := []float64{}
	for _, val := range va {
		switch v := val.(type) {
		case float64:
			res = append(res, v)
		case float32:
			res = append(res, float64(v))
		case int64:
			res = append(res, float64(v))
		case uint64:
			res = append(res, float64(v))
		case int32:
			res = append(res, float64(v))
		case uint32:
			res = append(res, float64(v))
		case int:
			res = append(res, float64(v))
		case int16:
			res = append(res, float64(v))
		case uint16:
			res = append(res, float64(v))
		case int8:
			res = append(res, float64(v))
		case uint8:
			res = append(res, float64(v))
		default:
			//convert to string first then, convert to integer
			vi, err := strconv.ParseFloat(o.asText(val), 64)
			if err == nil {
				res = append(res, float64(vi))
			}
		}
	}

	return res
}

//GetInt64 returns integer or default value if not exists
func (o *Options) GetInt64(key string, def int64) int64 {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return def
	}

	switch v := val.(type) {
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int64:
		return v
	case uint64:
		return int64(v)
	case int32:
		return int64(v)
	case uint32:
		return int64(v)
	case int:
		return int64(v)
	case int16:
		return int64(v)
	case uint16:
		return int64(v)
	case int8:
		return int64(v)
	case uint8:
		return int64(v)
	default:
		//convert to string first then, convert to integer
		vi, err := strconv.ParseInt(o.asText(val), 10, 64)
		if err != nil {
			return def
		}
		return vi
	}
}

//GetInt returns integer or default value if not exists
func (o *Options) GetInt(key string, def int) int {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return def
	}

	switch v := val.(type) {
	case float64:
		return int(v)
	case float32:
		return int(v)
	case int64:
		return int(v)
	case uint64:
		return int(v)
	case int32:
		return int(v)
	case uint32:
		return int(v)
	case int:
		return v
	case int16:
		return int(v)
	case uint16:
		return int(v)
	case int8:
		return int(v)
	case uint8:
		return int(v)
	default:
		//convert to string first then, convert to integer
		vi, err := strconv.Atoi(o.asText(val))
		if err != nil {
			return def
		}
		return vi
	}
}

//GetFloat returns decimal values or default value if not exist
func (o *Options) GetFloat(key string, def float64) float64 {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return def
	}

	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	case int32:
		return float64(v)
	case uint32:
		return float64(v)
	case int:
		return float64(v)
	case int16:
		return float64(v)
	case uint16:
		return float64(v)
	case int8:
		return float64(v)
	case uint8:
		return float64(v)
	default:
		//convert to string then to float
		vf, err := strconv.ParseFloat(o.asText(val), 64)
		if err != nil {
			return def
		}
		return vf
	}
}

//GetBool returns bool representation of given default value
func (o *Options) GetBool(key string, def bool) bool {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return def
	}

	//already in boolean
	vBool, ok := val.(bool)
	if ok {
		return vBool
	}

	//convert to string then to bool
	vb, err := strconv.ParseBool(o.asText(val))
	if err != nil {
		return def
	}
	return vb
}

//GetObject return value as interface
func (o *Options) GetObject(key string) (interface{}, bool) {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	return val, ok
}

//Get return map[string]interface{} item as options
func (o *Options) Get(key string) *Options {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	val, ok := container[key]
	if !ok {
		return New()
	}

	//is string with @
	vstr, ok := val.(string)
	if ok && strings.HasPrefix(vstr, "@") {
		filePath := o.getPath(vstr)
		relOpt, err := FromFile(filePath, "")
		if err == nil {
			return relOpt
		}
	}

	//assume the value is an object
	vMap, ok := val.(map[string]interface{})
	if !ok {
		return New()
	}

	newOpt := &Options{options: vMap}
	return newOpt
}

//IsEmpty return true if options having no values
func (o *Options) IsEmpty() bool {
	o.RLock()
	defer o.RUnlock()

	return len(o.options) == 0
}

//Exists return true if key exist in options
func (o *Options) Exists(key string) bool {
	o.RLock()
	defer o.RUnlock()

	container, key := o.getContainer(key)
	_, ok := container[key]

	return ok
}

//Set fill options with given key=val
func (o *Options) Set(key string, val interface{}) interface{} {
	o.Lock()
	defer o.Unlock()

	//navigate to container, if not exists, create one
	container, key := o.newContainer(key)
	ov, ok := container[key]
	container[key] = val

	if ok {
		return ov
	}
	return nil
}

//Assign configuration values
func (o *Options) Assign(optMap map[string]interface{}) {
	o.Lock()
	defer o.Unlock()

	o.options = make(map[string]interface{})
	for key, val := range optMap {
		o.options[key] = val
	}
}

//Parse option string given as key=value;opt2=value; ...
//Escape characters: \\ \r \n \t \= \; and \ followed by new-line
func (o *Options) Parse(params string) (err error) {
	o.Lock()
	defer o.Unlock()

	//parser state
	const (
		waitfieldDelim = 0
		waitoptDelim   = 1
	)

	//replace escaped endline (\ followed by new-line)
	params = rplLineEscape.Replace(params)

	//special characters
	ESCAPE := '\\'
	CR := '\r'

	//local variables for
	key := ""
	lastCh := CR
	state := waitfieldDelim

	//rune buffer and position
	n := len(params)
	if n > maxLen {
		msg := fmt.Sprintf("Key/Value len is greater than allowed limit (%d > %d)", n, maxLen)
		panic(msg)
	}
	rbuf := make([]rune, n)
	rp := 0

	//init map
	o.options = make(map[string]interface{})

parseLoop:
	for _, ch := range params {
		switch state {
		case waitfieldDelim:
			//escape sequence
			if lastCh == ESCAPE {
				//escape sequence
				if ch == delimField || ch == delimOpt {
					rbuf[rp] = ch
					rp++
				} else {
					rbuf[rp] = ESCAPE
					rp++
					rbuf[rp] = ch
					rp++
				}

				//escape char processed
				if ch == ESCAPE {
					lastCh = CR
					continue
				}
			} else if ch == delimOpt {
				err = errors.Errorf("unescaped character %c", ch)
				break parseLoop
			} else if ch == delimField {
				//got key
				key = string(rbuf[:rp])
				rp = 0

				state = waitoptDelim
				if len(key) == 0 {
					err = errors.New("empty key")
					break parseLoop
				}
			} else if ch != ESCAPE {
				//Valid rune, put into buffer
				rbuf[rp] = ch
				rp++
			}
		case waitoptDelim:
			if lastCh == ESCAPE {
				//escape sequence
				if ch == delimField || ch == delimOpt {
					rbuf[rp] = ch
					rp++
				} else {
					rbuf[rp] = ESCAPE
					rp++
					rbuf[rp] = ch
					rp++
				}

				//escape char processed
				if ch == ESCAPE {
					lastCh = CR
					continue
				}
			} else if ch == delimField {
				err = errors.Errorf("unescaped character %c", ch)
				break parseLoop
			} else if ch == delimOpt {
				//got value
				val := string(rbuf[:rp])
				rp = 0
				if len(val) == 0 {
					err = errors.Errorf("empty value for key %q", key)
					break parseLoop
				}

				//store options
				o.options[key] = val
				key = ""

				state = waitfieldDelim
			} else if ch != ESCAPE {
				//Valid rune, put into buffer
				rbuf[rp] = ch
				rp++
			}
		}

		//remember last character
		lastCh = ch
	}

	//Error, return
	if err != nil {
		o.options = make(map[string]interface{})
		return err
	}

	//latest value (key already found)
	if rp > 0 && state == waitoptDelim && len(key) > 0 {
		o.options[key] = string(rbuf[:rp])
	}

	return nil
}
