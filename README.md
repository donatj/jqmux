# jqmux

[![GoDoc](https://godoc.org/github.com/donatj/jqmux?status.svg)](https://godoc.org/github.com/donatj/jqmux)

An HTTP multiplexer, routing based on the requests JSON Body using [jq syntax](https://stedolan.github.io/jq/manual/) of JSON value filtering.

A particularly powerful usecase for this is webhook routing.

This utilizes the library [github.com/savaki/jq](https://github.com/savaki/jq) and more information about expected values can be found there.

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
