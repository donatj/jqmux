package jqmux

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/savaki/jq"
)

type handlerRecord struct {
	match   string
	handler http.Handler
}

type JqMux struct {
	handlers map[string][]handlerRecord
	ops      map[string]jq.Op
}

func NewMux() *JqMux {
	return &JqMux{
		handlers: make(map[string][]handlerRecord),
		ops:      make(map[string]jq.Op),
	}
}

func (mux *JqMux) Handle(pattern, match string, handler http.Handler) {
	if _, ok := mux.ops[pattern]; !ok {
		op, err := jq.Parse(pattern)
		if err != nil {
			panic(err)
		}

		mux.ops[pattern] = op
	}

	mux.handlers[pattern] = append(mux.handlers[pattern], handlerRecord{
		match, handler,
	})
}

func (mux *JqMux) HandleFunc(pattern, match string, handler func(http.ResponseWriter, *http.Request)) {
	mux.Handle(pattern, match, http.HandlerFunc(handler))
}

func (mux *JqMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var h http.Handler = nil

handlers:
	for p, m := range mux.handlers {
		_ = m
		op, _ := mux.ops[p]
		v, err := op.Apply(b)
		if err != nil {
			continue
		}

		vs := string(v)

		for _, hr := range m {
			if hr.match == vs {
				h = hr.handler
				break handlers
			}
		}
	}

	r.Body.Close()
	r.Body = ioutil.NopCloser(bytes.NewBuffer(b))

	if h != nil {
		h.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}
