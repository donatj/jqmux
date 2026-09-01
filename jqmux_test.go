package jqmux

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoBodyJSONMatchEmpty(t *testing.T) {
	req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte{}))
	rec := httptest.NewRecorder()

	x := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Error("write failed")
		}
	})

	jqm := NewMux()

	if err := jqm.Handle(".", "", x); err != nil {
		t.Fatal(err)
	}
	jqm.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d; got %d", http.StatusOK, res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if string(body) != "ok" {
		t.Errorf(`expected body "ok"; got: %s`, body)
	}
}

func TestBasicExample(t *testing.T) {
	mux := NewMux()

	if err := mux.HandleFunc(`.action`, `"opened"`, func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`it opened`)); err != nil {
			t.Error("write failed")
		}
	}); err != nil {
		t.Fatal(err)
	}

	if err := mux.HandleFunc(`.action`, `"synchronize"`, func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`it synchronized`)); err != nil {
			t.Error("write failed")
		}
	}); err != nil {
		t.Fatal(err)
	}

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
		defer res.Body.Close()
		if res.StatusCode != tc.status {
			t.Errorf("expected status %d; got %d", tc.status, res.StatusCode)
		}

		body, _ := io.ReadAll(res.Body)
		if string(body) != tc.output {
			t.Errorf(`expected body "%s"; got: "%s"`, tc.output, body)
		}
	}
}

func TestRequestBodyPreserved(t *testing.T) {
	const payload = `{"action":"opened"}`

	mux := NewMux()
	if err := mux.HandleFunc(`.action`, `"opened"`, func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(b)
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString(payload))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != payload {
		t.Errorf("handler received %q; want %q", got, payload)
	}
}

func TestOptionNotFoundHandler(t *testing.T) {
	mux := NewMux(OptionNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})))
	if err := mux.HandleFunc(`.action`, `"opened"`, func(http.ResponseWriter, *http.Request) {}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString(`{"action":"other"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusTeapot {
		t.Errorf("status = %d; want %d", got, http.StatusTeapot)
	}
}

func TestHandleInvalidPattern(t *testing.T) {
	mux := NewMux()
	if err := mux.Handle(`.action |`, `"opened"`, http.NotFoundHandler()); err == nil {
		t.Fatal("Handle returned nil error for an invalid jq pattern")
	}
}

func TestLargeNumberMatch(t *testing.T) {
	const number = "4722366482869645213696"

	mux := NewMux()
	if err := mux.HandleFunc(`.id`, number, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString(`{"id":`+number+`}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusNoContent {
		t.Errorf("status = %d; want %d", got, http.StatusNoContent)
	}
}
