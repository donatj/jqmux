# jqmux

[![CI](https://github.com/donatj/jqmux/actions/workflows/ci.yml/badge.svg)](https://github.com/donatj/jqmux/actions/workflows/ci.yml)
[![GoDoc](https://godoc.org/github.com/donatj/jqmux?status.svg)](https://godoc.org/github.com/donatj/jqmux)
[![Go Report Card](https://goreportcard.com/badge/github.com/donatj/jqmux)](https://goreportcard.com/report/github.com/donatj/jqmux)

An HTTP multiplexer which routes based on the incoming requests JSON body using the [jq syntax](https://stedolan.github.io/jq/manual/) of JSON value filtering.

A particularly fruitful usecase for this is webhook routing.

This uses [gojq](https://github.com/itchyny/gojq), a pure-Go jq implementation. Its supported syntax and behavioral differences from jq are documented in that project.

Filters are compiled when registered. `Handle` and `HandleFunc` panic when a filter is invalid, so configuration errors surface during application startup rather than while serving requests.

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
