//go:build wasm

package jsvalue

import (
	"syscall/js"

	. "webtyp.com/model"
)

// decodeBytes is the single contract for []byte decoding.
// Accepts JS string (UTF-8) or Uint8Array.
func decodeBytes(v js.Value) []byte {
	if v.Type() == js.TypeString {
		return []byte(v.String())
	}
	if v.InstanceOf(Uint8ArrayClass) {
		b := make([]byte, v.Length())
		js.CopyBytesToGo(b, v)
		return b
	}
	return nil
}

// encodeBytes is the single contract for []byte encoding — the write-side counterpart of
// decodeBytes. A Go string crosses to JS decoded from UTF-8, so string(val) silently
// replaces every byte that is not valid UTF-8 with U+FFFD — no error, no panic, just wrong
// data. []byte is arbitrary binary (a float32 vector, a hash, anything), so that substitution
// is the rule here, not the edge case. js.CopyBytesToJS is the only lossless path.
// decodeBytes already accepts a Uint8Array (verified above), so this is backward compatible.
func encodeBytes(val []byte) js.Value {
	arr := Uint8ArrayClass.New(len(val))
	js.CopyBytesToJS(arr, val)
	return arr
}

type jsObjectWriter struct {
	obj js.Value
}

func (w *jsObjectWriter) String(name, val string)        { w.obj.Set(name, val) }
func (w *jsObjectWriter) Int(name string, val int64)     { w.obj.Set(name, val) }
func (w *jsObjectWriter) Uint(name string, val uint64)   { w.obj.Set(name, val) }
func (w *jsObjectWriter) Float(name string, val float64) { w.obj.Set(name, val) }
func (w *jsObjectWriter) Bool(name string, val bool)     { w.obj.Set(name, val) }
func (w *jsObjectWriter) Bytes(name string, val []byte)  { w.obj.Set(name, encodeBytes(val)) }
func (w *jsObjectWriter) Null(name string)               { w.obj.Set(name, js.Null()) }
func (w *jsObjectWriter) Raw(name, val string)           { w.obj.Set(name, js.Global().Call("JSON.parse", val)) }

func (w *jsObjectWriter) Object(name string, val Encodable) {
	if val == nil || val.IsNil() {
		w.obj.Set(name, js.Null())
		return
	}
	sub := jsObject.New()
	sw := jsObjectWriter{obj: sub}
	val.EncodeFields(&sw)
	w.obj.Set(name, sub)
}

func (w *jsObjectWriter) Array(name string, n int) ArrayWriter {
	arr := jsArray.New(n)
	w.obj.Set(name, arr)
	return &jsArrayWriter{arr: arr, idx: 0}
}

type jsArrayWriter struct {
	arr js.Value
	idx int
}

func (w *jsArrayWriter) String(val string) { w.arr.SetIndex(w.idx, val); w.idx++ }
func (w *jsArrayWriter) Int(val int64)     { w.arr.SetIndex(w.idx, val); w.idx++ }
func (w *jsArrayWriter) Float(val float64) { w.arr.SetIndex(w.idx, val); w.idx++ }
func (w *jsArrayWriter) Bool(val bool)     { w.arr.SetIndex(w.idx, val); w.idx++ }
func (w *jsArrayWriter) Bytes(val []byte)  { w.arr.SetIndex(w.idx, encodeBytes(val)); w.idx++ }
func (w *jsArrayWriter) Close()            {}
func (w *jsArrayWriter) Object(val Encodable) {
	if val == nil || val.IsNil() {
		w.arr.SetIndex(w.idx, js.Null())
		w.idx++
		return
	}
	sub := jsObject.New()
	sw := jsObjectWriter{obj: sub}
	val.EncodeFields(&sw)
	w.arr.SetIndex(w.idx, sub)
	w.idx++
}

type jsObjectReader struct {
	obj js.Value
}

func (r *jsObjectReader) String(name string) (string, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return "", false
	}
	return v.String(), true
}

func (r *jsObjectReader) Int(name string) (int64, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return 0, false
	}
	return int64(v.Float()), true
}

func (r *jsObjectReader) Uint(name string) (uint64, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return 0, false
	}
	return uint64(v.Float()), true
}

func (r *jsObjectReader) Float(name string) (float64, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return 0, false
	}
	return v.Float(), true
}

func (r *jsObjectReader) Bool(name string) (bool, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return false, false
	}
	return v.Bool(), true
}

func (r *jsObjectReader) Bytes(name string) ([]byte, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return nil, false
	}
	return decodeBytes(v), true
}

func (r *jsObjectReader) Raw(name string) (string, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return "", false
	}
	return js.Global().Call("JSON.stringify", v).String(), true
}

func (r *jsObjectReader) Object(name string, into Decodable) bool {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return false
	}
	sr := jsObjectReader{obj: v}
	into.DecodeFields(&sr)
	return true
}

func (r *jsObjectReader) Array(name string) (ArrayReader, bool) {
	v := r.obj.Get(name)
	if v.IsUndefined() || v.IsNull() {
		return nil, false
	}
	return &jsArrayReader{arr: v}, true
}

type jsArrayReader struct {
	arr js.Value
}

func (r *jsArrayReader) Len() int { return r.arr.Length() }

func (r *jsArrayReader) String(i int) string { return r.arr.Index(i).String() }
func (r *jsArrayReader) Int(i int) int64     { return int64(r.arr.Index(i).Float()) }
func (r *jsArrayReader) Float(i int) float64 { return r.arr.Index(i).Float() }
func (r *jsArrayReader) Bool(i int) bool     { return r.arr.Index(i).Bool() }
func (r *jsArrayReader) Bytes(i int) []byte  { return decodeBytes(r.arr.Index(i)) }

func (r *jsArrayReader) Object(i int, into Decodable) bool {
	v := r.arr.Index(i)
	if v.IsUndefined() || v.IsNull() {
		return false
	}
	sr := jsObjectReader{obj: v}
	into.DecodeFields(&sr)
	return true
}
