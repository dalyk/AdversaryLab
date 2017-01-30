package protocol

import (
	"fmt"
	"reflect"

	"github.com/ugorji/go/codec"
)

// InterfaceExt handles custom (de)serialization of types to/from another interface{} value.
// The Encoder or Decoder will then handle the further (de)serialization of that known type.
//
// It is used by codecs (e.g. cbor, json) which use the format to do custom serialization of the types.

type Named interface {
	Name() string
}

type NamedType struct {
	Name  string
	Value interface{}
}

type RawNamedType struct {
	Name  string
	Value interface{}
}

type NamedTypeExt struct{}

func NamedTypeHandle() codec.Handle {
	var h *codec.CborHandle = new(codec.CborHandle)

	namedType := reflect.TypeOf(NamedType{})
	var namedTypeExt NamedTypeExt

	h.SetExt(namedType, 78, namedTypeExt)

	return h
}

func (x NamedTypeExt) WriteExt(interface{}) []byte {
	panic("unsupported")
}
func (x NamedTypeExt) ReadExt(interface{}, []byte) {
	panic("unsupported")
}

// ConvertExt converts a value into a simpler interface for easy encoding e.g. convert time.Time to int64.
// From NamedType to RawNamedType
func (x NamedTypeExt) ConvertExt(v interface{}) interface{} {
	//  fmt.Println("Converting Ext")
	switch v.(type) {
	case NamedType:
		var nt = v.(NamedType)
		return RawNamedType{Name: nt.Name, Value: nt.Value}
	// case *Named:
	//   var named Named = *v
	//   return NamedType{Name: named.Name(), Value: named}
	default:
		panic(fmt.Sprintf("unsupported format for named type conversion: expecting NamedType; got %T", v))
	}
}

// UpdateExt updates a value from a simpler interface for easy decoding e.g. convert int64 to time.Time.
// From NamedType to NamedType
func (x NamedTypeExt) UpdateExt(dest interface{}, v interface{}) {
	//	fmt.Println("Updating Ext")
	ret := dest.(*NamedType)
	switch v.(type) {
	case map[interface{}]interface{}:
		rnt := v.(map[interface{}]interface{})
		nt := NamedType{Name: rnt["Name"].(string), Value: rnt["Value"]}

		*ret = nt
	default:
		panic(fmt.Sprintf("unsupported format for named type update: expecting RawNamedType; got %T", v))
	}
}
