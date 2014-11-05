package ads

import (
	"reflect"
)

var idToType map[int8]reflect.Type
var typeToId map[reflect.Type]int8

var idToFunc map[int8]interface{}
var funcToId map[interface{}]int8

func init() {
	idToType = make(map[int8]reflect.Type)
	typeToId = make(map[reflect.Type]int8)

	idToFunc = make(map[int8]interface{})
	funcToId = make(map[interface{}]int8)
}

func RegisterType(id int8, instance interface{}) {
	typ := reflect.TypeOf(instance)

	if _, found := idToType[id]; found {
		panic(typ)
	}

	if _, found := typeToId[typ]; found {
		panic(typ)
	}

	idToType[id] = typ
	typeToId[typ] = id

	if id == FunctionId {
		panic(typ)
	}
}

func RegisterFunc(id int8, f interface{}) {
	ptr := reflect.ValueOf(f).Pointer()

	if _, found := idToFunc[id]; found {
		panic(f)
	}

	if _, found := funcToId[ptr]; found {
		panic(f)
	}

	idToFunc[id] = f
	funcToId[ptr] = id
}

func GetFuncId(f interface{}) int8 {
	return funcToId[reflect.ValueOf(f).Pointer()]
}
