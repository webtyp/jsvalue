//go:build wasm

package jsvalue

import (
	"syscall/js"

	"webtyp.com/fmt"
	. "webtyp.com/model"
)

var (
	jsObject = js.Global().Get("Object")
	jsArray  = js.Global().Get("Array")

	// Uint8ArrayClass is the JS Uint8Array constructor.
	Uint8ArrayClass = js.Global().Get("Uint8Array")
)

// ToJS converts Go values to JavaScript values recursively.
// For structs and slices, they must implement model.Encodable.
func ToJS(data any) js.Value {
	if data == nil {
		return js.Null()
	}

	switch v := data.(type) {
	case string:
		return js.ValueOf(v)
	case bool:
		return js.ValueOf(v)
	case int:
		return js.ValueOf(v)
	case int8:
		return js.ValueOf(int(v))
	case int16:
		return js.ValueOf(int(v))
	case int32:
		return js.ValueOf(int(v))
	case int64:
		return js.ValueOf(float64(v))
	case uint:
		return js.ValueOf(float64(v))
	case uint8:
		return js.ValueOf(int(v))
	case uint16:
		return js.ValueOf(int(v))
	case uint32:
		return js.ValueOf(float64(v))
	case uint64:
		return js.ValueOf(float64(v))
	case float32:
		return js.ValueOf(float64(v))
	case float64:
		return js.ValueOf(v)
	case []byte:
		return encodeBytes(v)
	case []any:
		arr := jsArray.New(len(v))
		for i, item := range v {
			arr.SetIndex(i, ToJS(item))
		}
		return arr
	case []string:
		arr := jsArray.New(len(v))
		for i, item := range v {
			arr.SetIndex(i, js.ValueOf(item))
		}
		return arr
	case []int:
		arr := jsArray.New(len(v))
		for i, item := range v {
			arr.SetIndex(i, js.ValueOf(item))
		}
		return arr
	case []int64:
		arr := jsArray.New(len(v))
		for i, item := range v {
			arr.SetIndex(i, js.ValueOf(float64(item)))
		}
		return arr
	case []float64:
		arr := jsArray.New(len(v))
		for i, item := range v {
			arr.SetIndex(i, js.ValueOf(item))
		}
		return arr
	default:
		if IsNil(data) {
			return js.Null()
		}
		if e, ok := data.(Encodable); ok {
			obj := jsObject.New()
			w := jsObjectWriter{obj: obj}
			e.EncodeFields(&w)
			return obj
		}
		// Fallback: convert to string
		return js.ValueOf(fmt.Convert(v).String())
	}
}

// ToGo converts JavaScript values to Go values.
// For structs and slices, they must implement model.Decodable.
func ToGo(jsVal js.Value, v any) error {
	if v == nil {
		return fmt.Err("jsvalue", "destination", "is", "nil")
	}

	switch ptr := v.(type) {
	case *any:
		*ptr = ToAny(jsVal)
	case *string:
		*ptr = jsVal.String()
	case *int:
		*ptr = jsVal.Int()
	case *int8:
		*ptr = int8(jsVal.Int())
	case *int16:
		*ptr = int16(jsVal.Int())
	case *int32:
		*ptr = int32(jsVal.Int())
	case *int64:
		*ptr = int64(jsVal.Float())
	case *uint:
		*ptr = uint(jsVal.Float())
	case *uint8:
		*ptr = uint8(jsVal.Int())
	case *uint16:
		*ptr = uint16(jsVal.Int())
	case *uint32:
		*ptr = uint32(jsVal.Float())
	case *uint64:
		*ptr = uint64(jsVal.Float())
	case *float32:
		*ptr = float32(jsVal.Float())
	case *float64:
		*ptr = jsVal.Float()
	case *bool:
		*ptr = jsVal.Bool()
	case *[]byte:
		*ptr = decodeBytes(jsVal)
	default:
		if IsNil(v) {
			return fmt.Err("jsvalue", "destination", "is", "nil")
		}
		if d, ok := v.(Decodable); ok {
			r := jsObjectReader{obj: jsVal}
			d.DecodeFields(&r)
			return nil
		}
		return fmt.Err("jsvalue", "unsupported", "destination", "type")
	}
	return nil
}

// ScanValue copies the JS value v into the Go pointer dest.
// Supports *string, *int, *int64, *int32, *float64, *bool, *[]byte, *any.
func ScanValue(v js.Value, dest any) error {
	return ToGo(v, dest)
}

// ToAny converts a JS value to a Go any.
// Integer numbers are returned as int64; non-integer numbers as float64.
// For objects, it returns the raw js.Value (no map).
func ToAny(v js.Value) any {
	switch v.Type() {
	case js.TypeNull, js.TypeUndefined:
		return nil
	case js.TypeBoolean:
		return v.Bool()
	case js.TypeNumber:
		f := v.Float()
		if f == float64(int64(f)) {
			return int64(f)
		}
		return f
	case js.TypeString:
		return v.String()
	case js.TypeObject:
		if v.InstanceOf(jsArray) {
			length := v.Length()
			arr := make([]any, length)
			for i := 0; i < length; i++ {
				arr[i] = ToAny(v.Index(i))
			}
			return arr
		}
		// Return raw js.Value for objects to avoid maps
		return v
	default:
		return v.String()
	}
}
