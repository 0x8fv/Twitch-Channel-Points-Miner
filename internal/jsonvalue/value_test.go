package jsonvalue

import (
	"encoding/json"
	"testing"
)

func TestStringAndBoolReturnOnlyTheirOwnType(t *testing.T) {
	if got := String("hello"); got != "hello" {
		t.Fatalf("String got %q", got)
	}
	for _, v := range []interface{}{nil, 5, true, []string{"a"}} {
		if got := String(v); got != "" {
			t.Fatalf("String(%v) got %q, want empty", v, got)
		}
	}
	if !Bool(true) || Bool(false) || Bool("true") || Bool(nil) {
		t.Fatalf("Bool should be true only for the bool true")
	}
}

func TestFloatAcceptsEveryJSONNumberShape(t *testing.T) {
	for v, want := range map[interface{}]float64{
		float64(2.5):        2.5,
		int(7):              7,
		json.Number("12.5"): 12.5,
		"3":                 0,
		nil:                 0,
	} {
		if got := Float(v); got != want {
			t.Fatalf("Float(%v) got %v, want %v", v, got, want)
		}
	}
}

func TestAtFollowsThePath(t *testing.T) {
	var data interface{}
	if err := json.Unmarshal([]byte(`{"data":{"user":{"id":"42","roles":null}}}`), &data); err != nil {
		t.Fatal(err)
	}
	if got := At(data, "data.user.id"); got != "42" {
		t.Fatalf("At got %v, want 42", got)
	}
	for _, path := range []string{"data.missing.id", "data.user.roles", "data.user.id.deeper", "nope"} {
		if got := At(data, path); got != nil {
			t.Fatalf("At(%q) got %v, want nil", path, got)
		}
	}
	if At(nil, "data") != nil {
		t.Fatalf("At on nil data should be nil")
	}
}

func TestFirstStringAndFirstIntSkipEmptyValues(t *testing.T) {
	data := map[string]interface{}{"a": "", "b": "second", "n": float64(0), "m": float64(9), "s": "text"}
	if got := FirstString(data, "missing", "a", "b"); got != "second" {
		t.Fatalf("FirstString got %q, want the first non-empty value", got)
	}
	if got := FirstString(data, "missing"); got != "" {
		t.Fatalf("FirstString got %q, want empty", got)
	}
	if got := FirstInt(data, "s", "n", "m"); got != 9 {
		t.Fatalf("FirstInt got %d, want the first non-zero number", got)
	}
	if FirstString(nil, "a") != "" || FirstInt(nil, "a") != 0 {
		t.Fatalf("a nil map should give zero values")
	}
}
