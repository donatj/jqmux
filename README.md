# jqmux

[![Build Status](https://travis-ci.org/donatj/jqmux.svg?branch=master)](https://travis-ci.org/donatj/jqmux)
[![GoDoc](https://godoc.org/github.com/donatj/jqmux?status.svg)](https://godoc.org/github.com/donatj/jqmux)
[![Go Report Card](https://goreportcard.com/badge/github.com/donatj/jqmux)](https://goreportcard.com/report/github.com/donatj/jqmux)

An HTTP multiplexer which routes based on the incoming requests JSON body using the [jq syntax](https://stedolan.github.io/jq/manual/) of JSON value filtering.

A particularly fruitful usecase for this is webhook routing.

## Limitations

This utilizes the library [github.com/savaki/jq](https://github.com/savaki/jq) for it's jq parsing. While it is very fast it is not fully featured, so more complicated jq queries might not work. More information about it's workings and expected values can be found there.

## Example

The first handler is executed if the body matches `{"action": "opened"}` whereas the second is executed if the body matches `{"action": "synchronize"}`

```go
package main

import (
	"net/http"

	"github.com/donatj/jqmux"
)

func main() {
	mux := jqmux.NewMux()

	mux.HandleFunc(`.action`, `"opened"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`body "action" was "opened"`))
	})

	mux.HandleFunc(`.action`, `"synchronize"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`body "action" was "synchronize"`))
	})

	http.ListenAndServe(":80", mux)
}
```
