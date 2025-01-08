package main

import (
	"reflect"
	"testing"
)

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		map1, map2, expected map[string][]interface{}
	}{
		{
			map1: map[string][]interface{}{
				"a": {"value1"},
				"b": {"value2"},
			},
			map2: map[string][]interface{}{
				"b": {"new_value2"},
				"c": {"value3"},
			},
			expected: map[string][]interface{}{
				"a": {"value1"},
				"b": {"new_value2"},
				"c": {"value3"},
			},
		},
		{
			map1: map[string][]interface{}{},
			map2: map[string][]interface{}{
				"a": {"value1"},
			},
			expected: map[string][]interface{}{
				"a": {"value1"},
			},
		},
		{
			map1: map[string][]interface{}{
				"a": {"value1"},
			},
			map2: map[string][]interface{}{},
			expected: map[string][]interface{}{
				"a": {"value1"},
			},
		},
		{
			map1:     map[string][]interface{}{},
			map2:     map[string][]interface{}{},
			expected: map[string][]interface{}{},
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			result := mergeMaps(test.map1, test.map2)
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("mergeMaps(%v, %v) = %v; want %v", test.map1, test.map2, result, test.expected)
			}
		})
	}
}
