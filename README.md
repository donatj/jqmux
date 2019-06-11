# jqmux

HTTP Mux based on JSON Body using jq Syntax

## Example

Given the HTTP Request Body

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

The first handler is executed if the body matches `{"action": "opened"}`

