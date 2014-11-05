package comp

import (
	"certcomp/ads"
	"reflect"
)

type C interface {
	Use(values ...ads.ADS)
	Call(f interface{}, args ...interface{}) []interface{}
}

type nilC struct{}

var Uses int64 = 0

func (c *nilC) Use(values ...ads.ADS) {
	for _, value := range values {
		Uses++
		value.AssertTransparent()
	}
}

func (c *nilC) Call(f interface{}, args ...interface{}) []interface{} {
	return Call(f, append(args, c))
}

var NilC = &nilC{}

func toValues(args []interface{}) []reflect.Value {
	values := make([]reflect.Value, len(args))
	for i, arg := range args {
		values[i] = reflect.ValueOf(arg)
	}
	return values
}

func fromValues(values []reflect.Value) []interface{} {
	ret := make([]interface{}, len(values))
	for i, value := range values {
		ret[i] = value.Interface()
	}
	return ret
}

func Call(f interface{}, args []interface{}) []interface{} {
	return fromValues(reflect.ValueOf(f).Call(toValues(args)))
}
