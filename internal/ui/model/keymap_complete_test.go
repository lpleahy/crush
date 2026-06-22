package model

import (
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
)

// TestDefaultKeyMap_AllBindingsHaveKeys walks the entire KeyMap and
// asserts every key.Binding resolves to at least one key. Because every
// field is built via common.Binding(group, action), an empty binding
// means a mistyped catalog group/action — this is the regression guard
// for every Style-A call site (global, editor, chat, initialize).
func TestDefaultKeyMap_AllBindingsHaveKeys(t *testing.T) {
	t.Parallel()

	bindingType := reflect.TypeOf(key.Binding{})

	var walk func(v reflect.Value, path string)
	walk = func(v reflect.Value, path string) {
		if v.Type() == bindingType {
			b := v.Interface().(key.Binding)
			if len(b.Keys()) == 0 {
				t.Errorf("binding %s has no keys (mistyped catalog group/action?)", path)
			}
			return
		}
		if v.Kind() == reflect.Struct {
			for i := 0; i < v.NumField(); i++ {
				walk(v.Field(i), path+"."+v.Type().Field(i).Name)
			}
		}
	}

	walk(reflect.ValueOf(DefaultKeyMap(nil)), "KeyMap")
}
