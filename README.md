# opt

Package for reading configuration from JSON or HJSON writen in Golang. 
For reading HJSON, [https://github.com/hjson/hjson-go](github.com/hjson/hjson-go) is used.

Usage example:

```go
package main

import (
	"github.com/ipsusila/opt"
)

func main() {
	//Open configuration from HJSON file
	options, err := opt.FromFile("config.hjson", opt.FormatHJSON)
	if err != nil {
		log.Fatal(err)
	}
	
	//Read string
	str := options.GetString("url", "https://golang.org")
	
	//Read sub configuration
	dbOpt := options.Get("db")
	username := dbOpt.GetString("username", "postgres")
	port := dbOpt.GetInt("port", 5432)
}
```

## Godoc
https://godoc.org/github.com/ipsusila/opt