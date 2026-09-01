// Package jqmux routes HTTP requests according to their JSON bodies using jq
// filters.
//
// A JqMux evaluates registered filters against a request body and dispatches
// the request when a filter's first result matches a registered JSON value.
// For example, the filter ".action" with the match value `"opened"` matches
// the body `{"action":"opened"}`.
//
// Register handlers before serving requests. Invalid filters panic during
// registration, while invalid JSON and unmatched requests are sent to the
// configured not-found handler.
package jqmux

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/itchyny/gojq"
)

// Option configures a JqMux created by NewMux.
type Option func(*JqMux)

type handlerRecord struct {
	match   string
	handler http.Handler
}

// JqMux is an HTTP request multiplexer that routes on JSON request bodies.
//
// Each registered jq filter is compiled once and reused for subsequent
// requests. JqMux restores the request body before invoking a matched or
// not-found handler, so handlers can read it normally.
type JqMux struct {
	handlers map[string][]handlerRecord
	codes    map[string]*gojq.Code

	errorHandler    func(error) http.Handler
	notFoundHandler http.Handler
}

// OptionErrorHandler sets the handler factory used when JqMux cannot read a
// request body.
func OptionErrorHandler(handler func(error) http.Handler) Option {
	return func(mux *JqMux) {
		mux.errorHandler = handler
	}
}

// OptionNotFoundHandler sets the handler called when a request body is invalid
// JSON or does not match any registered rule.
func OptionNotFoundHandler(handler http.Handler) Option {
	return func(mux *JqMux) {
		mux.notFoundHandler = handler
	}
}

// DefaultErrorHandler returns the default handler for request-body read errors.
// It responds with the error text and status 500.
func DefaultErrorHandler(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})
}

// DefaultNotFoundHandler writes the standard HTTP 404 response.
func DefaultNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// NewMux returns a new JqMux configured with DefaultErrorHandler and
// DefaultNotFoundHandler. Options override those defaults.
func NewMux(options ...Option) *JqMux {
	mux := &JqMux{
		handlers: make(map[string][]handlerRecord),
		codes:    make(map[string]*gojq.Code),

		errorHandler:    DefaultErrorHandler,
		notFoundHandler: http.HandlerFunc(DefaultNotFoundHandler),
	}

	for _, option := range options {
		option(mux)
	}

	return mux
}

// Handle registers handler for a jq pattern and match value.
//
// The first value emitted by pattern is JSON-encoded and compared with match.
// Match must therefore use JSON syntax: strings include quotes, while numbers,
// booleans, and null do not. Handle panics if pattern cannot be parsed or
// compiled.
func (mux *JqMux) Handle(pattern, match string, handler http.Handler) {
	if _, ok := mux.codes[pattern]; !ok {
		query, err := gojq.Parse(pattern)
		if err != nil {
			panic(err)
		}

		code, err := gojq.Compile(query)
		if err != nil {
			panic(err)
		}

		mux.codes[pattern] = code
	}

	mux.handlers[pattern] = append(mux.handlers[pattern], handlerRecord{
		match, handler,
	})
}

// HandleFunc registers handler as a function with the same behavior as Handle.
func (mux *JqMux) HandleFunc(pattern, match string, handler func(http.ResponseWriter, *http.Request)) {
	mux.Handle(pattern, match, http.HandlerFunc(handler))
}

// ServeHTTP routes r according to its JSON body. It restores the body before
// invoking the selected handler or the configured not-found handler.
func (mux *JqMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		mux.errorHandler(err).ServeHTTP(w, r)
		return
	}

	var input any
	validJSON := len(b) == 0 || json.Valid(b)
	if validJSON && len(b) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(b))
		decoder.UseNumber()
		if err := decoder.Decode(&input); err != nil {
			validJSON = false
		}
	}
	var h http.Handler

	if validJSON {
	handlers:
		for p, m := range mux.handlers {
			vs := ""
			if len(b) > 0 {
				iter := mux.codes[p].Run(input)
				v, ok := iter.Next()
				if !ok {
					continue
				}
				if _, ok := v.(error); ok {
					continue
				}

				result, err := json.Marshal(v)
				if err != nil {
					continue
				}
				vs = string(result)
			}

			for _, hr := range m {
				if hr.match == vs {
					h = hr.handler
					break handlers
				}
			}
		}
	}

	restoreBody(r, b)

	if h != nil {
		h.ServeHTTP(w, r)
		return
	}

	mux.notFoundHandler.ServeHTTP(w, r)
}

func restoreBody(r *http.Request, b []byte) {
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(b))
}
