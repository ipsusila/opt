package opt

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
	hjson "github.com/hjson/hjson-go"
)

func TestOptions(t *testing.T) {
	//first option
	txt := "key=value;pass=abcd123s;"
	op := New()

	err := op.Parse(txt)
	if err != nil {
		t.Error(err)
	}
	t.Logf("@Options: \n%s", op.Format("\n"))
	t.Logf(" -->%s", op.String())

	//second without ;
	txt = "key=value;pass=abcd123s;kv=xxxx"
	err = op.Parse(txt)
	if err != nil {
		t.Error(err)
	}
	t.Logf("@Options: \n%s", op.Format("\n"))
	t.Logf(" -->%s", op.String())

	//third option
	txt = "key=value;pass=abcd123s\\; \\ntest;kv=invalid"
	err = op.Parse(txt)
	if err != nil {
		t.Error(err)
	}
	t.Logf("@Options: \n%s", op.Format("\n"))
	t.Logf(" -->%s", op.String())

	//fourth option
	txt = "key=value;pass=abcd123s\\; test;kv=escaped\\\\dd\\=;  "
	err = op.Parse(txt)
	if err != nil {
		t.Error(err)
	}
	t.Logf("@Options: \n%s", op.Format("\n"))
	t.Logf(" -->%s", op.String())

	//fifth option
	txt = `key=value;pass=abcd123s\; \
a		test;kv=escaped\\dd\=;`

	err = op.Parse(txt)
	if err != nil {
		t.Error(err)
	}
	t.Logf("@Options: \n%s", op.Format("\n"))
	t.Logf(" -->%s", op.String())
}

func TestHjson(t *testing.T) {
	sampleText := []byte(`
    {
        # specify rate in requests/second
        rate: 1000
        array: [10]
		password: abcdeasdfd
    }`)

	cfgText := []byte(`
	{
		port: /dev/ttyAMA0
		baudrate: 2400
		readTimeout: 3 #timeout in seconds
		unit: uSv/h
		decimalPlace: 4
		minLength: 13
		timeZone: Asia/Jakarta
		endLine: a\r\nb
	}`)

	var dat map[string]interface{}
	if err := hjson.Unmarshal(sampleText, &dat); err != nil {
		t.Error(err)
	}

	op := New()
	op.Assign(dat)
	t.Logf("@Options: \n%s", op.Format("\n"))

	obj, _ := op.GetObject("array")
	fmt.Printf("@Options: %v, %T\n", op, obj)

	vi := op.GetFloat64Array("array")
	fmt.Printf("@Options: %+v, %T\n", vi, vi)

	var mcfg map[string]interface{}
	if err := hjson.Unmarshal(cfgText, &mcfg); err != nil {
		t.Error(err)
	}
	op.Assign(mcfg)
	t.Logf("@Config: \n%s", op.Format("\n"))

}

func TestHjsonOnly(t *testing.T) {
	text := []byte(`
	{
		YDOC_AWS-Batan-L02m_L02m:
		[
			{id: 11, name: "Temp"}
			{id: 20, name: "Hum"}
			{id: 30, name: "Speed"}
		]
		
		YDOC_AWS-Batan-L10m_L10m:
		[
			{id: 31, name: "Temp"}
			{id: 32, name: "Hum"}
			{id: 33, name: "Delta T"}
		]
		
		YDOC_AWS-Batan-L60m_L60m:
		[
			{id: 41, name: "Temp"}
			{id: 42, name: "Hum"}
			{id: 43, name: "Rain"}
		]
	}`)

	var dat map[string]interface{}
	if err := hjson.Unmarshal(text, &dat); err != nil {
		t.Fatal(err)
	}

	spew.Dump(dat)
	t.Logf("%+v", dat)

}

func TestHjsonOptions(t *testing.T) {
	cfgText := []byte(`
	{
		addr: localhost:21
		user: anonymous
		pass: abcd@abcd
		readTimeout: 3 #timeout in seconds
		checkInterval: 10 #every ten seconds
		timeZone: Asia/Jakarta
		
		# Sensors mapping for each altitude
		# Valid: Temp, Hum, Press, Direct, Speed, Rain, Solrad, DeltaT, Bat, Direct STD, Speed STD
		sensors: {
			YDOC_AWS-Batan-L02m_L02m: {
				Hum: 10
				Temp: 20
			}
			YDOC_AWS-Batan-L10m_L10m: {
				"Direct STD": 50
				Rain: 300
			}
		}
	}
	`)

	var dat map[string]interface{}

	// Decode and a check for errors.
	if err := hjson.Unmarshal(cfgText, &dat); err != nil {
		t.Error(err)
	}

	spew.Dump(dat)

	op := New()
	op.Assign(dat)
	t.Logf("@Config: \n%s", op.Format("\n"))

	sensors, ok := op.GetObject("sensors")
	if !ok {
		//
		t.Logf("Log failed")
		t.Fail()
	}

	sensMap, ok := sensors.(map[string]interface{})
	if !ok {
		t.Logf("Cast 1 failed")
		t.Fail()
	}

	for prefix, def := range sensMap {
		defMap, ok := def.(map[string]interface{})
		if !ok {
			t.Logf("Cast 2 failed")
			t.Fail()
		}

		t.Log(prefix)
		for sens, id := range defMap {
			id64 := int64(id.(float64))
			t.Logf("%s=%v, %03d", sens, id, id64)
		}
	}

	topt := op.Get("sensors")
	spew.Printf("Dump sensors %v", topt)
	spew.Dump(topt)
}

func TestGetOptions(t *testing.T) {
	cfgText := `{
		vInt: 10
		vInt64: 456789
		vFloat: 6.542134
		vFloat2: 1e+4
		vBool: true
		vStr: Ini string
	}`

	op, err := FromText(cfgText, FormatHJSON)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	t.Logf("vInt=%v", op.GetInt("vInt", 0))
	t.Logf("vInt64=%v", op.GetInt64("vInt64", 0))
	t.Logf("vFloat=%v", op.GetFloat("vFloat", 0.0))
	t.Logf("vFloat2=%v", op.GetFloat("vFloat2", 0))
	t.Logf("vBool=%v", op.GetBool("vBool", false))

	//test string
	t.Logf("vStr=%v, %v", op.GetString("vStr", "abcd"), op.GetInt("vStr", 20))
}

func TestFunctionality(t *testing.T) {
	cfgText := `
	{
		version: v1.0.0
		server: {
			id: conoco01
			listenAddr: ":8081"
			acceptTimeout: 10	# timeout (dalam detik)
			writeTimeout: 10 	# timeout (dalam detik)
			maxReceive: 50
			maxSend: 50
			device: ruptela
		}
		
		ruptela: {
			message: "http://localhost:8081/api/"
			report: "http://localhost:8081/reportAPI"
			appendIMEI: false
			serviceTimeout: 10  # REST Service timeout (dalam detik)
		}

		gpsdata: {
			storage: sql
			sql: {
				driver: postgres
				connection: "user=isi-user password=isi-password dbname=OSLOGREC_MTRACK host=127.0.0.1 port=5432 connect_timeout=30 sslmode=disable"
				maxIdle: 10			#Max jumlah koneksi idle
				maxOpen: 10			#Max jumlah koneksi open
				maxLifetime: 60		#Maximum lifetime (dalam detik)
				insertQuery: INSERT INTO "GPS_DATA"("IMEI", "DATA_LOG", "FLAG", "DATE_INS", "DESC", "GPS_CODE", "DATA_LEN") VALUES($1, $2, $3, $4, $5, $6, $7)
			}
		}
		
		log: {
			#Known types: console, file, sql
			type: console, file, sql
			console: {
				# use default options
			}


			#for file (uncomment type)
			# type: file
			file: {
				name: ./applog.log
				append: true
			}

			#for sql (uncomment type)
			# SQLite -> driver: sqlite3
			# PostgreSQL -> driver: postgres
			# type: sql
			sql: {
				driver: sqlite3
				connection: ./applog.sqlite
				maxIdle: 10			#Max jumlah koneksi idle
				maxOpen: 10			#Max jumlah koneksi open
				maxLifetime: 60		#Maximum lifetime (dalam detik)
				createQuery: CREATE TABLE applog(id INTEGER PRIMARY KEY AUTOINCREMENT, ts DATETIME, app VARCHAR(100), content TEXT)
				insertQuery: INSERT INTO applog(ts, app, content) VALUES(?, ?, ?)
			}
		}
	}
	`

	op, err := FromText(cfgText, FormatHJSON)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	//get version, expected v1.0.0
	ver := op.GetString("version", "")
	if ver != "v1.0.0" {
		t.Fatalf("Expecting  paramerer is 'v1.0.0' got '%v'", ver)
	}

	server := op.Get("server")
	if server.IsEmpty() {
		t.Fatalf("Options should not empty")
	}

	id1 := server.GetString("id", "id1")
	id2 := op.GetString("server.id", "id2")
	if id1 != id2 {
		t.Fatalf("%v <> %v (must be equal)", id1, id2)
	}
	t.Logf("server Id: %v == %v", id1, id2)

	ok := op.Exists("log.http")
	if ok {
		t.Fatalf("log.http must not exists")
	}

	//test
	ov := op.Set("log.http", "http://google.com")
	nv := op.GetString("log.http", "failed")
	t.Logf("Old value = %v, new value = %v", ov, nv)

	oldName := op.Set("log.file.name", "newlog.txt")
	newName := op.GetString("log.file.name", "failed")
	t.Logf("Old value = %v, new value = %v", oldName, newName)

	mFile := op.Get("log.file")
	t.Logf("mFile: %+v", mFile)

	eMap := op.Get("log.service")
	t.Logf("eMap: '%+v'", eMap)

	op.Set("log.service.api", "http://apiend.com")
	sMap := op.Get("log.service")
	t.Logf("sMap: '%+v'", sMap)

	//dump contents
	t.Log(op.AsJSON())

	//as string?
	t.Logf("String: %s", op)

	//format?
	t.Logf("Format: %v", op.Format("\n"))
}
