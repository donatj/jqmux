package jqmux

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoBodyJSONMatchEmpty(t *testing.T) {
	req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte{}))
	rec := httptest.NewRecorder()

	x := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	jqm := NewMux()

	jqm.Handle(".", "", x)
	jqm.ServeHTTP(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d; got %d", http.StatusOK, res.StatusCode)
	}

	body, _ := ioutil.ReadAll(res.Body)
	if string(body) != "ok" {
		t.Errorf(`expected body "ok"; got: %s`, body)
	}
}

func TestBasicExample(t *testing.T) {
	mux := NewMux()

	mux.HandleFunc(`.action`, `"opened"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`it opened`))
	})

	mux.HandleFunc(`.action`, `"synchronize"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`it synchronized`))
	})

	tt := []struct {
		body   string
		output string
		status int
	}{
		{`{"action": "opened"}`, "it opened", http.StatusOK},
		{`{"action": "synchronize"}`, "it synchronized", http.StatusOK},
		{`{"action": "FooBar"}`, "404 page not found\n", http.StatusNotFound},

		{``, "404 page not found\n", http.StatusNotFound},
		{`{"invalid json"`, "404 page not found\n", http.StatusNotFound},
		{`"string"`, "404 page not found\n", http.StatusNotFound},
	}

	for _, tc := range tt {
		req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		res := rec.Result()
		if res.StatusCode != tc.status {
			t.Errorf("expected status %d; got %d", tc.status, res.StatusCode)
		}

		body, _ := ioutil.ReadAll(res.Body)
		if string(body) != tc.output {
			t.Errorf(`expected body "%s"; got: "%s"`, tc.output, body)
		}
	}
}
