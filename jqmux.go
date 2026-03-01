// Package jqmux offers an HTTP multiplexer which routes based on the incoming
// requests JSON body using the jq syntax of JSON value filtering
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
	queries  map[string]*gojq.Query

	errorHandler    func(error) http.Handler
	notFoundHandler http.Handler
}

// OptionErrorHandler configures a custom error handler
func OptionErrorHandler(handler func(error) http.Handler) Option {
	return func(mux *JqMux) {
		mux.errorHandler = handler
	}
}

// OptionNotFoundHandler configures the http.Hander called on no matches
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
		queries:  make(map[string]*gojq.Query),

		errorHandler:    DefaultErrorHandler,
		notFoundHandler: http.HandlerFunc(DefaultNotFoundHandler),
	}

	for _, option := range options {
		option(mux)
	}

	return mux
}

// Handle registers the handler for the given pattern and match value.
// If the given jq pattern does not compile, Handle panics.
func (mux *JqMux) Handle(pattern, match string, handler http.Handler) {
	if _, ok := mux.queries[pattern]; !ok {
		query, err := gojq.Parse(pattern)
		if err != nil {
			panic(err)
		}

		mux.queries[pattern] = query
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

	var input interface{}
	jsonErr := json.Unmarshal(b, &input)

	var h http.Handler

handlers:
	for p, m := range mux.handlers {
		var vs string

		if len(b) == 0 || jsonErr != nil {
			vs = ""
		} else {
			query := mux.queries[p]
			iter := query.Run(input)
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

	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(b))

	if h != nil {
		h.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}
