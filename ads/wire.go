package ads

import (
	"bytes"
	"certcomp/sha"
	"encoding/binary"
	"io"
	"reflect"
)

type Hashable interface {
	ComputeHash() sha.Hash
}

var pool []*bytes.Buffer

func GetFromPool() *bytes.Buffer {
	n := len(pool)
	if n > 0 {
		r := pool[n-1]
		pool = pool[:n-1]

		r.Reset()
		return r
	} else {
		return new(bytes.Buffer)
	}
}

func ReturnToPool(b *bytes.Buffer) {
	pool = append(pool, b)
}

func Hash(v ADS) sha.Hash {
	if h := v.CachedHash(); h != nil {
		return *h
	}

	hashable, ok := v.(Hashable)

	var hash sha.Hash
	if ok {
		hash = hashable.ComputeHash()
	} else {
		buffer := GetFromPool()
		defer ReturnToPool(buffer)

		e := Encoder{
			Writer:      buffer,
			Transparent: map[ADS]bool{v: true},
		}
		e.Encode(&v)
		hash = sha.Sum(buffer.Bytes())
	}

	v.SetCachedHash(hash)
	return hash
}

type Encodeable interface {
	Encode(*Encoder)
	Decode(*Decoder)
}

type Encoder struct {
	io.Writer
	Transparent map[ADS]bool
}

const FunctionId = -127

func (e *Encoder) writeLE(value interface{}) {
	binary.Write(e, binary.LittleEndian, value)
}

func (e *Encoder) encodePtr(v reflect.Value) {
	if value, ok := v.Interface().(ADS); ok {
		if _, found := e.Transparent[value]; !found {
			e.Write([]byte{0})
			e.Write(Hash(value).Bytes())
			return
		} else {
			e.Write([]byte{1})
		}
	}

	if encodeable, ok := v.Interface().(Encodeable); ok {
		encodeable.Encode(e)
		return
	}

	e.encode(v.Elem())
}

func (e *Encoder) encode(v reflect.Value) {
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.Kind() == reflect.Ptr && v.IsNil() {
			e.Write([]byte{0})
			return
		} else if v.Kind() == reflect.Interface && !v.Elem().IsValid() {
			e.Write([]byte{0})
			return
		} else {
			e.Write([]byte{1})
		}
	}

	switch v.Kind() {
	case reflect.Bool, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:

		e.writeLE(v.Interface())

	case reflect.String:
		e.writeLE(int32(v.Len()))
		e.Write([]byte(v.String()))

	case reflect.Slice:
		e.writeLE(int32(v.Len()))
		fallthrough
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			e.encode(v.Index(i))
		}

	case reflect.Ptr:
		e.encodePtr(v)

	case reflect.Interface:
		if v.Elem().Kind() == reflect.Func {
			e.writeLE(int8(FunctionId))
			id, found := funcToId[v.Elem().Pointer()]
			if !found {
				panic(v.Elem())
			}
			e.writeLE(id)
		} else {
			id, found := typeToId[v.Elem().Type()]
			if !found {
				panic(v.Elem().Type())
			}
			e.Write([]byte{byte(id)})
			e.encode(v.Elem())
		}

	case reflect.Struct:
		typ := v.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Type == baseValueType && field.Anonymous {
				continue
			}

			e.encode(v.Field(i))
		}

	default: // Map, Func?
		panic(v)
	}
}

func (e *Encoder) Encode(ptrToValue interface{}) {
	v := reflect.ValueOf(ptrToValue)
	if v.Kind() != reflect.Ptr {
		panic(v)
	}

	e.encode(v.Elem())
}

type Decoder struct {
	io.Reader
}

var baseValueType = reflect.TypeOf(Base{})

func (d *Decoder) readLE(value interface{}) {
	binary.Read(d, binary.LittleEndian, value)
}

func (d *Decoder) decodePtr(v reflect.Value) {
	if value, ok := v.Interface().(ADS); ok {
		var typ int8
		d.readLE(&typ)

		if typ == 0 {
			var bytes [32]byte
			d.Read(bytes[:])
			value.SetCachedHash(sha.Hash(bytes))
			value.MakeOpaque()
			return
		}
	}

	if encodeable, ok := v.Interface().(Encodeable); ok {
		encodeable.Decode(d)
		return
	}

	d.decode(v.Elem())
}

func (d *Decoder) decode(v reflect.Value) {
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		var typ int8
		d.readLE(&typ)
		if typ == 0 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
	}

	switch v.Kind() {
	case reflect.Bool, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:

		d.readLE(v.Addr().Interface())

	case reflect.String:
		var length int32
		d.readLE(&length)
		buffer := make([]byte, length)
		v.SetString(string(buffer))

	case reflect.Slice:
		var length int32
		d.readLE(&length)
		v.Set(reflect.MakeSlice(v.Type(), int(length), int(length)))
		fallthrough
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			d.decode(v.Index(i))
		}

	case reflect.Ptr:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		d.decodePtr(v)

	case reflect.Interface:
		var id int8
		d.readLE(&id)

		if id == FunctionId {
			d.readLE(&id)
			f, found := idToFunc[id]
			if !found {
				panic(id)
			}
			v.Set(reflect.ValueOf(f))

		} else {
			typ, found := idToType[id]
			if !found {
				panic(id)
			}

			if v.Elem().IsValid() {
				d.decode(v.Elem())
			} else {
				actual := reflect.New(typ)
				d.decode(actual.Elem())
				v.Set(actual.Elem())
			}
		}

	case reflect.Struct:
		typ := v.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Type == baseValueType && field.Anonymous {
				continue
			}

			d.decode(v.Field(i))
		}

	default: // Map, Func?
		panic(v)

	}
}

func (d *Decoder) Decode(ptrToValue interface{}) {
	v := reflect.ValueOf(ptrToValue)
	if v.Kind() != reflect.Ptr {
		panic(v)
	}

	d.decode(v.Elem())
}

func Equals(a, b ADS) bool {
	return Hash(a) == Hash(b) && reflect.TypeOf(a) == reflect.TypeOf(b)
}

func collectChildrenPtr(v reflect.Value, children *[]ADS) {
	if value, ok := v.Interface().(ADS); ok {
		*children = append(*children, value)
	} else {
		collectChildren(v.Elem(), children)
	}
}

func collectChildren(v reflect.Value, children *[]ADS) {
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.Kind() == reflect.Ptr && v.IsNil() {
			return
		} else if v.Kind() == reflect.Interface && !v.Elem().IsValid() {
			return
		}
	}

	switch v.Kind() {
	case reflect.Bool, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return

	case reflect.String:
		return

	case reflect.Slice:
		fallthrough
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			collectChildren(v.Index(i), children)
		}

	case reflect.Ptr:
		collectChildrenPtr(v, children)

	case reflect.Interface:
		if v.Elem().Kind() == reflect.Func {
			return
		} else {
			if _, found := typeToId[v.Elem().Type()]; !found {
				panic(v.Elem().Type())
			}
			collectChildren(v.Elem(), children)
		}

	case reflect.Struct:
		typ := v.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Type == baseValueType && field.Anonymous {
				continue
			}

			collectChildren(v.Field(i), children)
		}

	default: // Map, Func?
		panic(v)
	}
}

func MakeOpaque(value ADS) {
	v := reflect.ValueOf(value)
	for v.Kind() != reflect.Struct {
		v = v.Elem()
	}

	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type == baseValueType && field.Anonymous {
			continue
		}

		v.Field(i).Set(reflect.Zero(field.Type))
	}

	value.MakeOpaque()
}

type Collectable interface {
	CollectChildren() []ADS
}

func CollectChildren(value ADS) []ADS {
	if collectable, ok := value.(Collectable); ok {
		return collectable.CollectChildren()
	}

	children := make([]ADS, 0, 8)

	v := reflect.ValueOf(value)
	for v.Kind() != reflect.Struct {
		v = v.Elem()
	}

	collectChildren(v, &children)

	return children
}
