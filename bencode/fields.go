// SPDX-FileCopyrightText: Copyright (c) 2026 autobrr
// SPDX-License-Identifier: MIT

package bencode

import (
	"reflect"
	"slices"
	"strings"
	"sync"
)

type structField struct {
	key             string
	index           []int
	omitEmpty       bool
	ignoreTypeError bool
}

type structInfo struct {
	fields []structField
	byKey  map[string]structField
}

var structInfoCache sync.Map

func structInfoFor(t reflect.Type) *structInfo {
	if cached, ok := structInfoCache.Load(t); ok {
		return cached.(*structInfo)
	}
	cached, _ := structInfoCache.LoadOrStore(t, buildStructInfo(t))
	return cached.(*structInfo)
}

func buildStructInfo(t reflect.Type) *structInfo {
	var collected []structField
	walkStructFields(t, nil, map[reflect.Type]bool{}, &collected)

	byKey := make(map[string]structField, len(collected))
	for _, f := range collected {
		if prev, ok := byKey[f.key]; ok && len(prev.index) <= len(f.index) {
			continue
		}
		byKey[f.key] = f
	}

	fields := make([]structField, 0, len(byKey))
	for _, f := range byKey {
		fields = append(fields, f)
	}
	slices.SortFunc(fields, func(a, b structField) int {
		return strings.Compare(a.key, b.key)
	})
	return &structInfo{fields: fields, byKey: byKey}
}

func walkStructFields(t reflect.Type, index []int, visited map[reflect.Type]bool, out *[]structField) {
	if visited[t] {
		return
	}
	visited[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		key, opts := parseTag(f.Tag.Get("bencode"))
		if key == "-" {
			continue
		}
		idx := append(append([]int(nil), index...), i)
		if f.Anonymous && key == "" {
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				walkStructFields(ft, idx, visited, out)
				continue
			}
		}
		if key == "" {
			key = f.Name
		}
		*out = append(*out, structField{
			key:             key,
			index:           idx,
			omitEmpty:       slices.Contains(opts, "omitempty"),
			ignoreTypeError: slices.Contains(opts, "ignore_unmarshal_type_error"),
		})
	}
}

func parseTag(tag string) (string, []string) {
	parts := strings.Split(tag, ",")
	return parts[0], parts[1:]
}

func fieldForDecode(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

func fieldForEncode(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}
