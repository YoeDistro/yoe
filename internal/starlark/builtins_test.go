package starlark

import (
	"math"
	"math/big"
	"testing"

	"go.starlark.net/starlark"
)

func TestStarlarkToGo_None(t *testing.T) {
	got, err := starlarkToGo(starlark.None)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestStarlarkToGo_Float(t *testing.T) {
	got, err := starlarkToGo(starlark.Float(3.14))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("got %T, want float64", got)
	}
	if f != 3.14 {
		t.Errorf("got %v, want 3.14", f)
	}
}

func TestStarlarkToGo_NestedList(t *testing.T) {
	inner := starlark.NewList([]starlark.Value{
		starlark.String("a"),
		starlark.MakeInt(2),
	})
	outer := starlark.NewList([]starlark.Value{
		inner,
		starlark.Bool(true),
	})
	got, err := starlarkToGo(outer)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	list, ok := got.([]any)
	if !ok {
		t.Fatalf("got %T, want []any", got)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	innerList, ok := list[0].([]any)
	if !ok {
		t.Fatalf("inner got %T, want []any", list[0])
	}
	if len(innerList) != 2 {
		t.Fatalf("inner len = %d, want 2", len(innerList))
	}
	if innerList[0] != "a" {
		t.Errorf("innerList[0] = %v, want %q", innerList[0], "a")
	}
	if innerList[1] != int64(2) {
		t.Errorf("innerList[1] = %v, want 2", innerList[1])
	}
	if list[1] != true {
		t.Errorf("list[1] = %v, want true", list[1])
	}
}

func TestStarlarkToGo_NestedDict(t *testing.T) {
	inner := starlark.NewDict(1)
	if err := inner.SetKey(starlark.String("x"), starlark.MakeInt(1)); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	outer := starlark.NewDict(1)
	if err := outer.SetKey(starlark.String("nested"), inner); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	got, err := starlarkToGo(outer)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", got)
	}
	nested, ok := m["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested got %T, want map[string]any", m["nested"])
	}
	if nested["x"] != int64(1) {
		t.Errorf("nested[x] = %v, want 1", nested["x"])
	}
}

func TestStarlarkToGo_IntOverflow(t *testing.T) {
	// Build a starlark.Int beyond int64 range using big.Int.
	n := new(big.Int).SetInt64(math.MaxInt64)
	n.Add(n, new(big.Int).SetInt64(1))
	v := starlark.MakeBigInt(n)
	got, err := starlarkToGo(v)
	if err == nil {
		t.Fatalf("expected error for int overflow, got value %v", got)
	}
}

func TestStarlarkToGo_DictNonStringKey(t *testing.T) {
	d := starlark.NewDict(1)
	if err := d.SetKey(starlark.MakeInt(42), starlark.String("v")); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	got, err := starlarkToGo(d)
	if err == nil {
		t.Fatalf("expected error for non-string dict key, got value %v", got)
	}
}

func TestStarlarkToGo_UnsupportedTuple(t *testing.T) {
	tup := starlark.Tuple{starlark.String("a"), starlark.MakeInt(1)}
	got, err := starlarkToGo(tup)
	if err == nil {
		t.Fatalf("expected error for tuple, got value %v", got)
	}
}

func TestStarlarkToGo_NestedErrorPropagation(t *testing.T) {
	// A list containing an int-overflow value — outer call must surface the error.
	n := new(big.Int).SetInt64(math.MaxInt64)
	n.Add(n, new(big.Int).SetInt64(1))
	overflow := starlark.MakeBigInt(n)
	list := starlark.NewList([]starlark.Value{starlark.String("ok"), overflow})
	got, err := starlarkToGo(list)
	if err == nil {
		t.Fatalf("expected nested error to propagate, got value %v", got)
	}
}
