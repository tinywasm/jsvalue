//go:build wasm

package jsvalue

import (
	"reflect"
	"syscall/js"

	. "github.com/tinywasm/fmt"
)

var (
	jsObject = js.Global().Get("Object")
	jsArray  = js.Global().Get("Array")
)

// ToJS converts Go values to JavaScript values recursively.
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
		return js.ValueOf(int(v))
	case uint:
		return js.ValueOf(int(v))
	case uint8:
		return js.ValueOf(int(v))
	case uint16:
		return js.ValueOf(int(v))
	case uint32:
		return js.ValueOf(int(v))
	case uint64:
		return js.ValueOf(int(v))
	case float32:
		return js.ValueOf(float64(v))
	case float64:
		return js.ValueOf(v)
	case []byte:
		return js.ValueOf(string(v))
	case []any:
		arr := jsArray.New(len(v))
		for i, item := range v {
			arr.SetIndex(i, ToJS(item))
		}
		return arr
	case map[string]any:
		obj := jsObject.New()
		for key, val := range v {
			obj.Set(key, ToJS(val))
		}
		return obj
	case map[string]string:
		obj := jsObject.New()
		for key, val := range v {
			obj.Set(key, js.ValueOf(val))
		}
		return obj
	case map[string]int:
		obj := jsObject.New()
		for key, val := range v {
			obj.Set(key, js.ValueOf(val))
		}
		return obj
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
	default:
		// Use reflection to handle structs and slices
		val := reflect.ValueOf(data)

		// Handle pointers - dereference and recurse
		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return js.Null()
			}
			return ToJS(val.Elem().Interface())
		}

		// Handle slices of any type using reflection
		if val.Kind() == reflect.Slice {
			arr := jsArray.New(val.Len())
			for i := 0; i < val.Len(); i++ {
				arr.SetIndex(i, ToJS(val.Index(i).Interface()))
			}
			return arr
		}

		// Handle structs
		if val.Kind() == reflect.Struct {
			obj := jsObject.New()
			typ := val.Type()
			for i := 0; i < val.NumField(); i++ {
				field := typ.Field(i)
				jsonTag := field.Tag.Get("json")
				if jsonTag == "" {
					jsonTag = field.Name
				}
				// Handle "omitempty" and other tag options if needed, but for now basic name support
				if jsonTag == "-" {
					continue
				}

				// Simple tag parsing to get the name
				if idx := Index(jsonTag, ","); idx != -1 {
					jsonTag = jsonTag[:idx]
				}

				obj.Set(jsonTag, ToJS(val.Field(i).Interface()))
			}
			return obj
		}

		// Fallback: convert to string
		return js.ValueOf(Convert(v).String())
	}
}

// ToGo converts JavaScript values to Go values.
func ToGo(jsVal js.Value, v any) error {
	// Basic implementation - extend as needed
	switch ptr := v.(type) {
	case *any:
		*ptr = ToAny(jsVal)
	case *map[string]any:
		if *ptr == nil {
			*ptr = make(map[string]any)
		}
		keys := jsObject.Call("keys", jsVal)
		length := keys.Length()
		for i := 0; i < length; i++ {
			key := keys.Index(i).String()
			(*ptr)[key] = ToAny(jsVal.Get(key))
		}
	case *[]any:
		length := jsVal.Length()
		if cap(*ptr) < length {
			*ptr = make([]any, length)
		} else {
			*ptr = (*ptr)[:length]
		}
		for i := 0; i < length; i++ {
			(*ptr)[i] = ToAny(jsVal.Index(i))
		}
	case *string:
		*ptr = jsVal.String()
	case *int:
		*ptr = jsVal.Int()
	case *float64:
		*ptr = jsVal.Float()
	case *bool:
		*ptr = jsVal.Bool()
	case *byte: // byte is alias for uint8
		*ptr = byte(jsVal.Int())
	case *[]byte:
		// []byte is encoded as string, so decode from string
		if jsVal.Type() == js.TypeString {
			*ptr = []byte(jsVal.String())
			return nil
		}
		if jsVal.InstanceOf(jsArray) {
			length := jsVal.Length()
			*ptr = make([]byte, length)
			for i := 0; i < length; i++ {
				(*ptr)[i] = byte(jsVal.Index(i).Int())
			}
			return nil
		}
	default:
		// Use reflection to handle pointers to slices and structs
		val := reflect.ValueOf(v)
		if val.Kind() != reflect.Ptr || val.IsNil() {
			return Err("jsvalue: destination must be a non-nil pointer")
		}
		elem := val.Elem()

		// Handle slices of any type
		if elem.Kind() == reflect.Slice {
			sliceType := elem.Type()
			elemType := sliceType.Elem()

			// Special case: []byte is encoded as string, not array
			// Check if this is a []byte (slice of uint8)
			if elemType.Kind() == reflect.Uint8 {
				if jsVal.Type() == js.TypeString {
					elem.SetBytes([]byte(jsVal.String()))
					return nil
				}
			}

			if !jsVal.InstanceOf(jsArray) {
				return nil // Not an array, ignore or return error?
			}

			length := jsVal.Length()

			// Create new slice with correct capacity
			newSlice := reflect.MakeSlice(sliceType, length, length)

			for i := 0; i < length; i++ {
				jsItem := jsVal.Index(i)

				// Special case for [][]byte: inner []byte is encoded as string
				if elemType.Kind() == reflect.Slice && elemType.Elem().Kind() == reflect.Uint8 {
					if jsItem.Type() == js.TypeString {
						newSlice.Index(i).SetBytes([]byte(jsItem.String()))
						continue
					}
				}

				// Create new element of the correct type
				newElem := reflect.New(elemType)

				// Recursively decode into the new element
				if err := ToGo(jsItem, newElem.Interface()); err != nil {
					return err
				}

				// Set the decoded value into the slice
				newSlice.Index(i).Set(newElem.Elem())
			}

			// Set the new slice to the target
			elem.Set(newSlice)
			return nil
		}

		// Handle structs
		if elem.Kind() == reflect.Struct {
			// Check if jsVal is actually an object (not string, number, etc.)
			if jsVal.Type() != js.TypeObject {
				return nil // Cannot decode non-object into struct
			}

			typ := elem.Type()
			for i := 0; i < elem.NumField(); i++ {
				field := typ.Field(i)
				jsonTag := field.Tag.Get("json")
				if jsonTag == "" {
					jsonTag = field.Name
				}
				if jsonTag == "-" {
					continue
				}
				if idx := Index(jsonTag, ","); idx != -1 {
					jsonTag = jsonTag[:idx]
				}

				jsField := jsVal.Get(jsonTag)
				if !jsField.IsUndefined() && !jsField.IsNull() {
					// Recursively decode
					fieldVal := elem.Field(i)
					if fieldVal.CanAddr() {
						ToGo(jsField, fieldVal.Addr().Interface())
					}
				}
			}
		}
	}
	return nil
}


// ScanValue copies the JS value v into the Go pointer dest.
// Supports *string, *int, *int64, *int32, *float64, *bool, *[]byte, *any.
// []byte is read from a Uint8Array via js.CopyBytesToGo.
func ScanValue(v js.Value, dest any) error {
	switch p := dest.(type) {
	case *string:
		*p = v.String()
	case *int:
		*p = v.Int()
	case *int64:
		*p = int64(v.Float())
	case *int32:
		*p = int32(v.Int())
	case *float64:
		*p = v.Float()
	case *bool:
		*p = v.Bool()
	case *[]byte:
		ua := Uint8ArrayClass.New(v)
		b := make([]byte, ua.Length())
		js.CopyBytesToGo(b, ua)
		*p = b
	case *any:
		*p = ToAny(v)
	default:
		return Errf("jsvalue: unsupported scan type: %T", dest)
	}
	return nil
}

// ToAny converts a JS value to a Go any.
// Integer numbers are returned as int64; non-integer numbers as float64.
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
		obj := make(map[string]any)
		keys := jsObject.Call("keys", v)
		length := keys.Length()
		for i := 0; i < length; i++ {
			key := keys.Index(i).String()
			obj[key] = ToAny(v.Get(key))
		}
		return obj
	default:
		return v.String()
	}
}
