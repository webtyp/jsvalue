//go:build wasm

package jsvalue

import (
	"syscall/js"
	"testing"

	. "webtyp.com/model"
)

type TestStruct struct {
	Name    string
	Age     int
	Ignored string
	Default string
}

func (s *TestStruct) IsNil() bool { return s == nil }
func (s *TestStruct) EncodeFields(w FieldWriter) {
	w.String("name", s.Name)
	w.Int("age", int64(s.Age))
	w.String("Default", s.Default)
}
func (s *TestStruct) DecodeFields(r FieldReader) {
	s.Name, _ = r.String("name")
	age, _ := r.Int("age")
	s.Age = int(age)
	s.Default, _ = r.String("Default")
}

type MultiArrayStruct struct {
	A []int
	B []string
}

func (s *MultiArrayStruct) IsNil() bool { return s == nil }
func (s *MultiArrayStruct) EncodeFields(w FieldWriter) {
	aw1 := w.Array("a", len(s.A))
	for _, v := range s.A {
		aw1.Int(int64(v))
	}
	aw1.Close()
	aw2 := w.Array("b", len(s.B))
	for _, v := range s.B {
		aw2.String(v)
	}
	aw2.Close()
}
func (s *MultiArrayStruct) DecodeFields(r FieldReader) {
	if ar, ok := r.Array("a"); ok {
		s.A = make([]int, ar.Len())
		for i := 0; i < ar.Len(); i++ {
			s.A[i] = int(ar.Int(i))
		}
	}
	if ar, ok := r.Array("b"); ok {
		s.B = make([]string, ar.Len())
		for i := 0; i < ar.Len(); i++ {
			s.B[i] = ar.String(i)
		}
	}
}

type ComplexStruct struct {
	Nested *TestStruct
	List   []int
}

func (s *ComplexStruct) IsNil() bool { return s == nil }
func (s *ComplexStruct) EncodeFields(w FieldWriter) {
	w.Object("nested", s.Nested)
	aw := w.Array("list", len(s.List))
	for _, v := range s.List {
		aw.Int(int64(v))
	}
	aw.Close()
}
func (s *ComplexStruct) DecodeFields(r FieldReader) {
	s.Nested = &TestStruct{}
	r.Object("nested", s.Nested)
	if ar, ok := r.Array("list"); ok {
		s.List = make([]int, ar.Len())
		for i := 0; i < ar.Len(); i++ {
			s.List[i] = int(ar.Int(i))
		}
	}
}

func TestToJS(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		validate func(js.Value) bool
	}{
		{"nil", nil, func(v js.Value) bool { return v.IsNull() }},
		{"string", "hello", func(v js.Value) bool { return v.String() == "hello" }},
		{"int", 123, func(v js.Value) bool { return v.Int() == 123 }},
		{"int64", int64(999), func(v js.Value) bool { return v.Int() == 999 }},
		{"int64_large", int64(1) << 40, func(v js.Value) bool { return int64(v.Float()) == int64(1)<<40 }},
		{"uint32_max", uint32(4294967295), func(v js.Value) bool { return uint32(v.Float()) == 4294967295 }},
		{"float64", 1.5, func(v js.Value) bool { return v.Float() == 1.5 }},
		{"float32", float32(1.25), func(v js.Value) bool { return v.Float() == 1.25 }},
		{"bool", true, func(v js.Value) bool { return v.Bool() == true }},
		{"bytes", []byte("xyz"), func(v js.Value) bool {
			if !v.InstanceOf(Uint8ArrayClass) {
				return false
			}
			got := make([]byte, v.Length())
			js.CopyBytesToGo(got, v)
			return string(got) == "xyz"
		}},
		{"slice_any", []any{1, "a"}, func(v js.Value) bool {
			return v.Length() == 2 && v.Index(0).Int() == 1 && v.Index(1).String() == "a"
		}},
		{"slice_string", []string{"a", "b"}, func(v js.Value) bool {
			return v.Length() == 2 && v.Index(0).String() == "a"
		}},
		{"struct", &TestStruct{Name: "Alice", Age: 30}, func(v js.Value) bool {
			return v.Get("name").String() == "Alice" && v.Get("age").Int() == 30
		}},
		{"multi_array_struct", &MultiArrayStruct{A: []int{1, 2}, B: []string{"x", "y"}}, func(v js.Value) bool {
			return v.Get("a").Length() == 2 && v.Get("b").Length() == 2 &&
				v.Get("a").Index(0).Int() == 1 && v.Get("b").Index(0).String() == "x"
		}},
		{"nil_pointer", (*TestStruct)(nil), func(v js.Value) bool { return v.IsNull() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := ToJS(tt.input)
			if !tt.validate(val) {
				t.Errorf("ToJS validation failed for %v", tt.name)
			}
		})
	}
}

// TestBytes_RoundTrip_NonUTF8 is the real bug this package had: string(val) on encode
// silently replaces every byte that isn't valid UTF-8 with U+FFFD. This is the exact
// non-UTF-8 vector MASTER_PLAN.md's fase 0 uses for the same round-trip requirement at the
// IndexedDB layer — 0x00, 0xFF, an overlong encoding (0xC0 0x80), and a malformed surrogate
// pair, byte for byte.
func TestBytes_RoundTrip_NonUTF8(t *testing.T) {
	original := make([]byte, 1024)
	for i := range original {
		original[i] = byte(i % 256)
	}
	// Force specific invalid-UTF-8 patterns in, not just relying on the modulo sequence to
	// contain them incidentally.
	original[0], original[1] = 0x00, 0xFF
	original[2], original[3] = 0xC0, 0x80 // overlong encoding of NUL
	original[4], original[5] = 0xED, 0xA0 // malformed surrogate-pair lead

	val := ToJS(original)
	if !val.InstanceOf(Uint8ArrayClass) {
		t.Fatalf("ToJS(non-UTF-8 []byte) did not produce a Uint8Array")
	}

	var got []byte
	if err := ToGo(val, &got); err != nil {
		t.Fatalf("ToGo: %v", err)
	}

	if len(got) != len(original) {
		t.Fatalf("round trip length = %d, want %d", len(got), len(original))
	}
	for i := range original {
		if got[i] != original[i] {
			t.Fatalf("byte %d: got %#x, want %#x (round trip corrupted non-UTF-8 data)", i, got[i], original[i])
		}
	}
}

func TestToGo(t *testing.T) {
	t.Run("Basic Types", func(t *testing.T) {
		val := ToJS("hello")
		var s string
		if err := ToGo(val, &s); err != nil {
			t.Fatal(err)
		}
		if s != "hello" {
			t.Errorf("want hello, got %s", s)
		}

		val = ToJS(123)
		var i int
		if err := ToGo(val, &i); err != nil {
			t.Fatal(err)
		}
		if i != 123 {
			t.Errorf("want 123, got %d", i)
		}

		var i64 int64
		if err := ToGo(val, &i64); err != nil {
			t.Fatal(err)
		}
		if i64 != 123 {
			t.Errorf("want 123, got %d", i64)
		}

		var u32 uint32
		val = ToJS(uint32(4294967295))
		if err := ToGo(val, &u32); err != nil {
			t.Fatal(err)
		}
		if u32 != 4294967295 {
			t.Errorf("want 4294967295, got %d", u32)
		}
	})

	t.Run("Multi Array Struct", func(t *testing.T) {
		input := &MultiArrayStruct{A: []int{1, 2}, B: []string{"x", "y"}}
		val := ToJS(input)
		var res MultiArrayStruct
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if len(res.A) != 2 || res.A[1] != 2 || len(res.B) != 2 || res.B[1] != "y" {
			t.Errorf("multi array struct mismatch: %+v", res)
		}
	})

	t.Run("Struct", func(t *testing.T) {
		ts := &TestStruct{Name: "Alice", Age: 30}
		val := ToJS(ts)
		var res TestStruct
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if res.Name != "Alice" || res.Age != 30 {
			t.Errorf("struct mismatch: %+v", res)
		}
	})

	t.Run("Complex Struct", func(t *testing.T) {
		cs := &ComplexStruct{
			Nested: &TestStruct{Name: "Diff", Age: 40},
			List:   []int{5, 6},
		}
		val := ToJS(cs)
		var res ComplexStruct
		if err := ToGo(val, &res); err != nil {
			t.Fatal(err)
		}
		if res.Nested.Name != "Diff" || len(res.List) != 2 {
			t.Errorf("complex struct mismatch: %+v", res)
		}
	})

	t.Run("[]byte", func(t *testing.T) {
		inputBytes := []byte("binary data")
		val := ToJS(inputBytes)
		var resBytes []byte
		if err := ToGo(val, &resBytes); err != nil {
			t.Fatal(err)
		}
		if string(resBytes) != "binary data" {
			t.Errorf("want 'binary data', got %s", string(resBytes))
		}

		// From Uint8Array
		ua := Uint8ArrayClass.New(3)
		ua.SetIndex(0, 1)
		ua.SetIndex(1, 2)
		ua.SetIndex(2, 3)
		var resBytes2 []byte
		if err := ToGo(ua, &resBytes2); err != nil {
			t.Fatal(err)
		}
		if len(resBytes2) != 3 || resBytes2[0] != 1 {
			t.Errorf("Uint8Array decode failed")
		}
	})
}

func TestScanValue(t *testing.T) {
	var s string
	if err := ScanValue(js.ValueOf("hello"), &s); err != nil || s != "hello" {
		t.Fatalf("string: got %q err %v", s, err)
	}
	var i int
	if err := ScanValue(js.ValueOf(42), &i); err != nil || i != 42 {
		t.Fatalf("int: got %d err %v", i, err)
	}
}

func TestToAny(t *testing.T) {
	if v := ToAny(js.Null()); v != nil {
		t.Fatalf("null: got %v", v)
	}
	if v := ToAny(js.ValueOf("hi")); v != "hi" {
		t.Fatalf("string: got %v", v)
	}
	// Object returns raw js.Value
	obj := jsObject.New()
	if v := ToAny(obj); !js.Value(v.(js.Value)).Equal(obj) {
		t.Fatal("expected raw js.Value for object")
	}
}

func TestCodecAllocations(t *testing.T) {
	// Note: syscall/js boxes every value passed to obj.Set/Get as interface{},
	// so Go-heap allocs here are inherent to the JS bridge, not to the codec itself.
	// json/binary achieve true 0-alloc; jsvalue cannot due to the JS bridge overhead.
	// This test documents the stable allocation count so regressions are caught.
	input := &TestStruct{Name: "Alice", Age: 30, Default: "x"}
	var out TestStruct

	encAllocs := testing.AllocsPerRun(100, func() {
		_ = ToJS(input)
	})
	// jsObjectWriter + syscall/js boxing: expect a small fixed number, not growing with data.
	if encAllocs > 10 {
		t.Errorf("ToJS(Encodable) allocated %v times, regression threshold is 10", encAllocs)
	}

	jsVal := ToJS(input)
	decAllocs := testing.AllocsPerRun(100, func() {
		out = TestStruct{}
		_ = ToGo(jsVal, &out)
	})
	if decAllocs > 10 {
		t.Errorf("ToGo(Decodable) allocated %v times, regression threshold is 10", decAllocs)
	}
	_ = out
}
