package jqmux

import (
	"bytes"
	"errors"
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

	jqm.Handle(".", "", x)
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

	mux.HandleFunc(`.action`, `"opened"`, func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`it opened`)); err != nil {
			t.Error("write failed")
		}
	})

	mux.HandleFunc(`.action`, `"synchronize"`, func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`it synchronized`)); err != nil {
			t.Error("write failed")
		}
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

// TestRequestBodyPreserved verifies the handler can still read the full request
// body after the mux has consumed it for routing.
func TestRequestBodyPreserved(t *testing.T) {
	const payload = `{"action":"opened"}`

	mux := NewMux()
	mux.HandleFunc(`.action`, `"opened"`, func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading body: %v", err)
			return
		}
		if _, err := w.Write(b); err != nil {
			t.Error("write failed")
		}
	})

	req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(payload)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	got, _ := io.ReadAll(res.Body)
	if string(got) != payload {
		t.Errorf("expected body %q; got %q", payload, got)
	}
}

// TestOptionNotFoundHandler verifies a custom not-found handler is called when
// no route matches.
func TestOptionNotFoundHandler(t *testing.T) {
	notFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "custom not found", http.StatusTeapot)
	})

	mux := NewMux(OptionNotFoundHandler(notFound))
	mux.HandleFunc(`.action`, `"opened"`, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(`{"action":"other"}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusTeapot {
		t.Errorf("expected status %d; got %d", http.StatusTeapot, res.StatusCode)
	}
}

// TestOptionErrorHandler verifies a custom error handler is called when the
// mux encounters an error reading the request body.
func TestOptionErrorHandler(t *testing.T) {
	var handledErr error
	errHandler := func(err error) http.Handler {
		handledErr = err
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "custom error", http.StatusBadRequest)
		})
	}

	mux := NewMux(OptionErrorHandler(errHandler))

	// Use an erroring reader to trigger the error handler.
	readErr := errors.New("read error")
	req, _ := http.NewRequest("POST", "localhost", &errReader{err: readErr})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d; got %d", http.StatusBadRequest, res.StatusCode)
	}
	if handledErr == nil {
		t.Error("expected error handler to be called")
	}
}

// errReader is an io.Reader that always returns an error.
type errReader struct{ err error }

func (e *errReader) Read([]byte) (int, error) { return 0, e.err }

// TestHandleInvalidPattern verifies that registering an invalid jq pattern
// does not panic, and instead causes requests to receive a 500 error.
func TestHandleInvalidPattern(t *testing.T) {
	mux := NewMux()
	mux.Handle("this is not valid jq %%", "x", http.NotFoundHandler())

	tt := []struct {
		name string
		body string
	}{
		{"valid JSON body", `{"foo":"bar"}`},
		{"empty body", ``},
		{"invalid JSON body", `{invalid`},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusInternalServerError {
				t.Errorf("expected status %d for invalid jq pattern; got %d", http.StatusInternalServerError, res.StatusCode)
			}
		})
	}
}

// TestNumericMatch verifies routing on numeric JSON values.
func TestNumericMatch(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(`.count`, `42`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("forty-two"))
	})

	tt := []struct {
		body   string
		output string
		status int
	}{
		{`{"count": 42}`, "forty-two", http.StatusOK},
		{`{"count": 0}`, "404 page not found\n", http.StatusNotFound},
	}

	for _, tc := range tt {
		req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != tc.status {
			t.Errorf("body=%q: expected status %d; got %d", tc.body, tc.status, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if string(body) != tc.output {
			t.Errorf("body=%q: expected %q; got %q", tc.body, tc.output, body)
		}
	}
}

// TestBooleanMatch verifies routing on boolean JSON values.
func TestBooleanMatch(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(`.active`, `true`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("active"))
	})
	mux.HandleFunc(`.active`, `false`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("inactive"))
	})

	tt := []struct {
		body   string
		output string
		status int
	}{
		{`{"active": true}`, "active", http.StatusOK},
		{`{"active": false}`, "inactive", http.StatusOK},
	}

	for _, tc := range tt {
		req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != tc.status {
			t.Errorf("body=%q: expected status %d; got %d", tc.body, tc.status, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if string(body) != tc.output {
			t.Errorf("body=%q: expected %q; got %q", tc.body, tc.output, body)
		}
	}
}

// TestNullMatch verifies routing when a field value is JSON null.
func TestNullMatch(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(`.value`, `null`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("null value"))
	})

	req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(`{"value": null}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d; got %d", http.StatusOK, res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if string(body) != "null value" {
		t.Errorf(`expected "null value"; got %q`, body)
	}
}

// TestNestedFieldMatch verifies routing on deeply-nested jq expressions.
func TestNestedFieldMatch(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(`.repository.name`, `"myrepo"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("matched repo"))
	})

	tt := []struct {
		body   string
		output string
		status int
	}{
		{`{"repository":{"name":"myrepo"}}`, "matched repo", http.StatusOK},
		{`{"repository":{"name":"other"}}`, "404 page not found\n", http.StatusNotFound},
		{`{"repository":{}}`, "404 page not found\n", http.StatusNotFound},
	}

	for _, tc := range tt {
		req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != tc.status {
			t.Errorf("body=%q: expected status %d; got %d", tc.body, tc.status, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if string(body) != tc.output {
			t.Errorf("body=%q: expected %q; got %q", tc.body, tc.output, body)
		}
	}
}

// TestArrayIndexMatch verifies routing using array-index jq expressions.
func TestArrayIndexMatch(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(`.items[0]`, `"first"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("got first"))
	})

	tt := []struct {
		body   string
		output string
		status int
	}{
		{`{"items":["first","second"]}`, "got first", http.StatusOK},
		{`{"items":["second","first"]}`, "404 page not found\n", http.StatusNotFound},
		{`{"items":[]}`, "404 page not found\n", http.StatusNotFound},
	}

	for _, tc := range tt {
		req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != tc.status {
			t.Errorf("body=%q: expected status %d; got %d", tc.body, tc.status, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if string(body) != tc.output {
			t.Errorf("body=%q: expected %q; got %q", tc.body, tc.output, body)
		}
	}
}

// TestInvalidJSONNotFound verifies that a non-empty invalid JSON body is routed
// to the not-found handler rather than being treated like an empty body.
func TestInvalidJSONNotFound(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(".", "", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("empty match"))
	})

	req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(`{invalid`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d for invalid JSON; got %d", http.StatusNotFound, res.StatusCode)
	}
}

// TestMultiValueIterator verifies that routes are matched against all values
// produced by a jq expression that yields multiple results (e.g. .items[]).
func TestMultiValueIterator(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(`.items[]`, `"target"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("found"))
	})

	tt := []struct {
		body   string
		output string
		status int
	}{
		{`{"items":["other","target"]}`, "found", http.StatusOK},
		{`{"items":["a","b","c"]}`, "404 page not found\n", http.StatusNotFound},
	}

	for _, tc := range tt {
		req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		if res.StatusCode != tc.status {
			res.Body.Close()
			t.Errorf("body=%q: expected status %d; got %d", tc.body, tc.status, res.StatusCode)
			continue
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if string(body) != tc.output {
			t.Errorf("body=%q: expected %q; got %q", tc.body, tc.output, body)
		}
	}
}

// TestMultiplePatterns verifies that multiple distinct jq patterns can be
// registered simultaneously and each routes correctly.
func TestMultiplePatterns(t *testing.T) {
	mux := NewMux()
	mux.HandleFunc(`.type`, `"push"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("push event"))
	})
	mux.HandleFunc(`.ref`, `"refs/heads/main"`, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("main branch"))
	})

	tt := []struct {
		body   string
		output string
		status int
	}{
		{`{"type":"push","ref":"refs/heads/feature"}`, "push event", http.StatusOK},
		{`{"type":"other","ref":"refs/heads/main"}`, "main branch", http.StatusOK},
		{`{"type":"other","ref":"refs/heads/feature"}`, "404 page not found\n", http.StatusNotFound},
	}

	for _, tc := range tt {
		req, _ := http.NewRequest("POST", "localhost", bytes.NewReader([]byte(tc.body)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		res := rec.Result()
		defer res.Body.Close()

		if res.StatusCode != tc.status {
			t.Errorf("body=%q: expected status %d; got %d", tc.body, tc.status, res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if string(body) != tc.output {
			t.Errorf("body=%q: expected %q; got %q", tc.body, tc.output, body)
		}
	}
}
