// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protoreflect

import (
	"fmt"
	"math"
	"reflect"
)

// Value is a union where only one Go type may be set at a time.
// The Value is used to represent all possible values a field may take.
// The following shows what Go type is used to represent each proto Kind:
//
//	+------------+-------------------------------------+
//	| Go type    | Protobuf kind                       |
//	+------------+-------------------------------------+
//	| bool       | BoolKind                            |
//	| int32      | Int32Kind, Sint32Kind, Sfixed32Kind |
//	| int64      | Int64Kind, Sint64Kind, Sfixed64Kind |
//	| uint32     | Uint32Kind, Fixed32Kind             |
//	| uint64     | Uint64Kind, Fixed64Kind             |
//	| float32    | FloatKind                           |
//	| float64    | DoubleKind                          |
//	| string     | StringKind                          |
//	| []byte     | BytesKind                           |
//	| EnumNumber | EnumKind                            |
//	+------------+-------------------------------------+
//	| Message    | MessageKind, GroupKind              |
//	| List       |                                     |
//	| Map        |                                     |
//	+------------+-------------------------------------+
//
// Multiple protobuf Kinds may be represented by a single Go type if the type
// can losslessly represent the information for the proto kind. For example,
// Int64Kind, Sint64Kind, and Sfixed64Kind are all represented by int64,
// but use different integer encoding methods.
//
// The List or Map types are used if the FieldDescriptor.Cardinality of the
// corresponding field is Repeated and a Map if and only if
// FieldDescriptor.IsMap is true.
//
// Converting to/from a Value and a concrete Go value panics on type mismatch.
// For example, ValueOf("hello").Int() panics because this attempts to
// retrieve an int64 from a string.
type Value value

// The protoreflect API uses a custom Value union type instead of interface{}
// to keep the future open for performance optimizations. Using an interface{}
// always incurs an allocation for primitives (e.g., int64) since it needs to
// be boxed on the heap (as interfaces can only contain pointers natively).
// Instead, we represent the Value union as a flat struct that internally keeps
// track of which type is set. Using unsafe, the Value union can be reduced
// down to 24B, which is identical in size to a slice.
//
// The latest compiler (Go1.11) currently suffers from some limitations:
//	• With inlining, the compiler should be able to statically prove that
//	only one of these switch cases are taken and inline one specific case.
//	See https://golang.org/issue/22310.

// ValueOf returns a Value initialized with the concrete value stored in v.
// This panics if the type does not match one of the allowed types in the
// Value union.
//
// After calling ValueOf on a []byte, the slice must no longer be mutated.
func ValueOf(v interface{}) Value {
	switch v := v.(type) {
	case nil:
		return Value{}
	case bool:
		if v {
			return Value{typ: boolType, num: 1}
		} else {
			return Value{typ: boolType, num: 0}
		}
	case int32:
		return Value{typ: int32Type, num: uint64(v)}
	case int64:
		return Value{typ: int64Type, num: uint64(v)}
	case uint32:
		return Value{typ: uint32Type, num: uint64(v)}
	case uint64:
		return Value{typ: uint64Type, num: uint64(v)}
	case float32:
		return Value{typ: float32Type, num: uint64(math.Float64bits(float64(v)))}
	case float64:
		return Value{typ: float64Type, num: uint64(math.Float64bits(float64(v)))}
	case string:
		return valueOfString(v)
	case []byte:
		return valueOfBytes(v[:len(v):len(v)])
	case EnumNumber:
		return Value{typ: enumType, num: uint64(v)}
	case Message, List, Map:
		return valueOfIface(v)
	default:
		// TODO: Special case ProtoEnum, ProtoMessage, *[]T, and *map[K]V?
		// Note: this would violate the documented invariant in Interface.
		panic(fmt.Sprintf("invalid type: %v", reflect.TypeOf(v)))
	}
}

// IsValid reports whether v is populated with a value.
func (v Value) IsValid() bool {
	return v.typ != nilType
}

// Interface returns v as an interface{}.
// Returned []byte values must not be mutated.
//
// Invariant: v == ValueOf(v).Interface()
func (v Value) Interface() interface{} {
	switch v.typ {
	case nilType:
		return nil
	case boolType:
		return v.Bool()
	case int32Type:
		return int32(v.Int())
	case int64Type:
		return int64(v.Int())
	case uint32Type:
		return uint32(v.Uint())
	case uint64Type:
		return uint64(v.Uint())
	case float32Type:
		return float32(v.Float())
	case float64Type:
		return float64(v.Float())
	case stringType:
		return v.String()
	case bytesType:
		return v.Bytes()
	case enumType:
		return v.Enum()
	default:
		return v.getIface()
	}
}

// Bool returns v as a bool and panics if the type is not a bool.
func (v Value) Bool() bool {
	switch v.typ {
	case boolType:
		return v.num > 0
	default:
		panic("proto: value type mismatch")
	}
}

// Int returns v as a int64 and panics if the type is not a int32 or int64.
func (v Value) Int() int64 {
	switch v.typ {
	case int32Type, int64Type:
		return int64(v.num)
	default:
		panic("proto: value type mismatch")
	}
}

// Uint returns v as a uint64 and panics if the type is not a uint32 or uint64.
func (v Value) Uint() uint64 {
	switch v.typ {
	case uint32Type, uint64Type:
		return uint64(v.num)
	default:
		panic("proto: value type mismatch")
	}
}

// Float returns v as a float64 and panics if the type is not a float32 or float64.
func (v Value) Float() float64 {
	switch v.typ {
	case float32Type, float64Type:
		return math.Float64frombits(uint64(v.num))
	default:
		panic("proto: value type mismatch")
	}
}

// String returns v as a string. Since this method implements fmt.Stringer,
// this returns the formatted string value for any non-string type.
func (v Value) String() string {
	switch v.typ {
	case stringType:
		return v.getString()
	default:
		return fmt.Sprint(v.Interface())
	}
}

// Bytes returns v as a []byte and panics if the type is not a []byte.
// The returned slice must not be mutated.
func (v Value) Bytes() []byte {
	switch v.typ {
	case bytesType:
		return v.getBytes()
	default:
		panic("proto: value type mismatch")
	}
}

// Enum returns v as a EnumNumber and panics if the type is not a EnumNumber.
func (v Value) Enum() EnumNumber {
	switch v.typ {
	case enumType:
		return EnumNumber(v.num)
	default:
		panic("proto: value type mismatch")
	}
}

// Message returns v as a Message and panics if the type is not a Message.
func (v Value) Message() Message {
	switch v := v.getIface().(type) {
	case Message:
		return v
	default:
		panic("proto: value type mismatch")
	}
}

// List returns v as a List and panics if the type is not a List.
func (v Value) List() List {
	switch v := v.getIface().(type) {
	case List:
		return v
	default:
		panic("proto: value type mismatch")
	}
}

// Map returns v as a Map and panics if the type is not a Map.
func (v Value) Map() Map {
	switch v := v.getIface().(type) {
	case Map:
		return v
	default:
		panic("proto: value type mismatch")
	}
}

// MapKey returns v as a MapKey and panics for invalid MapKey types.
func (v Value) MapKey() MapKey {
	switch v.typ {
	case boolType, int32Type, int64Type, uint32Type, uint64Type, stringType:
		return MapKey(v)
	}
	panic("proto: invalid map key type")
}

// MapKey is used to index maps, where the Go type of the MapKey must match
// the specified key Kind (see MessageDescriptor.IsMapEntry).
// The following shows what Go type is used to represent each proto Kind:
//
//	+---------+-------------------------------------+
//	| Go type | Protobuf kind                       |
//	+---------+-------------------------------------+
//	| bool    | BoolKind                            |
//	| int32   | Int32Kind, Sint32Kind, Sfixed32Kind |
//	| int64   | Int64Kind, Sint64Kind, Sfixed64Kind |
//	| uint32  | Uint32Kind, Fixed32Kind             |
//	| uint64  | Uint64Kind, Fixed64Kind             |
//	| string  | StringKind                          |
//	+---------+-------------------------------------+
//
// A MapKey is constructed and accessed through a Value:
//	k := ValueOf("hash").MapKey() // convert string to MapKey
//	s := k.String()               // convert MapKey to string
//
// The MapKey is a strict subset of valid types used in Value;
// converting a Value to a MapKey with an invalid type panics.
type MapKey value

// IsValid reports whether k is populated with a value.
func (k MapKey) IsValid() bool {
	return Value(k).IsValid()
}

// Interface returns k as an interface{}.
func (k MapKey) Interface() interface{} {
	return Value(k).Interface()
}

// Bool returns k as a bool and panics if the type is not a bool.
func (k MapKey) Bool() bool {
	return Value(k).Bool()
}

// Int returns k as a int64 and panics if the type is not a int32 or int64.
func (k MapKey) Int() int64 {
	return Value(k).Int()
}

// Uint returns k as a uint64 and panics if the type is not a uint32 or uint64.
func (k MapKey) Uint() uint64 {
	return Value(k).Uint()
}

// String returns k as a string. Since this method implements fmt.Stringer,
// this returns the formatted string value for any non-string type.
func (k MapKey) String() string {
	return Value(k).String()
}

// Value returns k as a Value.
func (k MapKey) Value() Value {
	return Value(k)
}
