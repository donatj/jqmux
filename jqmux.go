// Package jqmux offers an HTTP multiplexer which routes based on the incoming
// requests JSON body using the jq syntax of JSON value filtering.
package jqmux

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/itchyny/gojq"
)

// Option sets an option of the passed JqMux
type Option func(*JqMux)

type handlerRecord struct {
	match   string
	handler http.Handler
}

// JqMux is an HTTP request multiplexer.
// It matches the body of each incoming request against a list of registered
// jq patterns and calls the handler for the first pattern that
// matches given value.
type JqMux struct {
	handlers map[string][]handlerRecord
	codes    map[string]*gojq.Code

	errorHandler    func(error) http.Handler
	notFoundHandler http.Handler
}

// OptionErrorHandler configures a custom error handler
func OptionErrorHandler(handler func(error) http.Handler) Option {
	return func(mux *JqMux) {
		mux.errorHandler = handler
	}
}

// OptionNotFoundHandler configures the http.Handler called on no matches.
func OptionNotFoundHandler(handler http.Handler) Option {
	return func(mux *JqMux) {
		mux.notFoundHandler = handler
	}
}

// DefaultErrorHandler is the default error handler when calling NewMux
func DefaultErrorHandler(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})
}

// DefaultNotFoundHandler is the default http.Handler when calling NewMux
func DefaultNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// NewMux allocates and returns a new JqMux.
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

// Handle registers the handler for the given pattern and match value.
// It panics when the jq pattern cannot be compiled.
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

// HandleFunc is a convenience method which casts the given handler to
// http.HandlerFunc and registers the casted handler
func (mux *JqMux) HandleFunc(pattern, match string, handler func(http.ResponseWriter, *http.Request)) {
	mux.Handle(pattern, match, http.HandlerFunc(handler))
}

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
