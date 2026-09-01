# jqmux

[![CI](https://github.com/donatj/jqmux/actions/workflows/ci.yml/badge.svg)](https://github.com/donatj/jqmux/actions/workflows/ci.yml)
[![GoDoc](https://godoc.org/github.com/donatj/jqmux?status.svg)](https://godoc.org/github.com/donatj/jqmux)

An HTTP multiplexer which routes based on the incoming requests JSON body using the [jq syntax](https://stedolan.github.io/jq/manual/) of JSON value filtering.

A particularly useful case is webhook routing.

This uses [gojq](https://github.com/itchyny/gojq), a pure-Go jq implementation. Its supported syntax and behavioral differences from jq are documented in that project.

Filters are compiled when registered. `Handle` and `HandleFunc` panic when a filter is invalid, so configuration errors surface during application startup rather than while serving requests.

## Basic webhook routing

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

## Routing rules

Each rule runs a jq filter against the request body and compares its first
result with the registered match value. Match values use JSON syntax, so string
values include quotes while numbers, booleans, and `null` do not.

```go
// Matches {"repository":{"visibility":"private"}}.
mux.HandleFunc(`.repository.visibility`, `"private"`, handlePrivateRepository)

// Matches {"pull_request":{"merged":true}}.
mux.HandleFunc(`.pull_request.merged`, `true`, handleMergedPullRequest)

// Matches {"delivery":{"attempt":2}}.
mux.HandleFunc(`.delivery.attempt`, `2`, handleSecondAttempt)
```

Filters can also derive a value before it is compared. This rule handles an
event with no `action` field by treating it as `"unknown"`.

```go
mux.HandleFunc(`.action // "unknown"`, `"unknown"`, handleUnknownAction)
```

## Custom fallback handler

Use `OptionNotFoundHandler` to handle invalid JSON or bodies that do not match
any registered rule.

```go
mux := jqmux.NewMux(
	jqmux.OptionNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unsupported webhook", http.StatusUnprocessableEntity)
	})),
)

mux.HandleFunc(`.event`, `"push"`, handlePush)
```
