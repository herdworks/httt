package scripting

import "reflect"

func jsPlainValue(value any) any {
	return jsPlainReflectValue(reflect.ValueOf(value))
}

func jsPlainMap(value any) (map[string]any, bool) {
	plain, ok := jsPlainValue(value).(map[string]any)
	return plain, ok
}

func jsPlainReflectValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}

	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		switch value.Elem().Kind() {
		case reflect.Array, reflect.Interface, reflect.Map, reflect.Slice:
			return jsPlainReflectValue(value.Elem())
		default:
			return value.Interface()
		}
	}

	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return nil
		}
		if value.Type().Key().Kind() != reflect.String {
			return value.Interface()
		}

		plain := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			plain[iter.Key().String()] = jsPlainReflectValue(iter.Value())
		}
		return plain
	case reflect.Array:
		plain := make([]any, value.Len())
		for i := range plain {
			plain[i] = jsPlainReflectValue(value.Index(i))
		}
		return plain
	case reflect.Slice:
		plain := make([]any, value.Len())
		for i := range plain {
			plain[i] = jsPlainReflectValue(value.Index(i))
		}
		return plain
	default:
		return value.Interface()
	}
}
