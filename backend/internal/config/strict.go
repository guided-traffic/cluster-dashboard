package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// decodeStrict decodes a YAML document into out, a pointer to a struct whose
// fields carry yaml tags, and returns every problem it finds instead of
// stopping at the first: a key the struct does not name, a key set twice, a
// mapping, list or single value where another is expected, a value its field
// cannot hold. Each problem names the line and the key's path. The struct is
// the schema: a field is a known key, its type what the key holds.
func decodeStrict(data []byte, out any) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return []string{strings.TrimPrefix(err.Error(), "yaml: ")}
	}
	if len(doc.Content) == 0 {
		return nil // an empty file sets nothing
	}
	var d decoder
	d.walk(doc.Content[0], reflect.ValueOf(out).Elem(), "")
	return d.problems
}

type decoder struct {
	problems []string
}

func (d *decoder) add(node *yaml.Node, path, message string) {
	if path == "" {
		path = "the document"
	}
	d.problems = append(d.problems, fmt.Sprintf("line %d: %s: %s", node.Line, path, message))
}

var durationType = reflect.TypeFor[time.Duration]()

func (d *decoder) walk(node *yaml.Node, v reflect.Value, path string) {
	if node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	if isNull(node) {
		return // an explicit null leaves the default in place
	}
	switch {
	case v.Type() == durationType:
		d.scalar(node, v, path, "a duration such as 20s or 5m")
	case v.Kind() == reflect.Pointer:
		elem := reflect.New(v.Type().Elem())
		d.walk(node, elem.Elem(), path)
		v.Set(elem)
	case v.Kind() == reflect.Struct:
		d.mapping(node, v, path)
	case v.Kind() == reflect.Slice:
		d.list(node, v, path)
	case v.Kind() == reflect.Map:
		d.dictionary(node, v, path)
	default:
		d.scalar(node, v, path, describe(v.Type()))
	}
}

// mapping walks a mapping into a struct, key by key.
func (d *decoder) mapping(node *yaml.Node, v reflect.Value, path string) {
	if node.Kind != yaml.MappingNode {
		d.add(node, path, "must be a mapping of keys")
		return
	}
	fields := fieldsOf(v.Type())
	seen := map[string]bool{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		keyPath := join(path, key.Value)
		switch index, known := fields[key.Value]; {
		case seen[key.Value]:
			d.add(key, keyPath, "is set twice")
		case !known:
			d.add(key, keyPath, "is not a known key")
		default:
			d.walk(value, v.Field(index), keyPath)
		}
		seen[key.Value] = true
	}
}

// list walks a sequence into a slice, item by item.
func (d *decoder) list(node *yaml.Node, v reflect.Value, path string) {
	if node.Kind != yaml.SequenceNode {
		d.add(node, path, "must be a list")
		return
	}
	s := reflect.MakeSlice(v.Type(), len(node.Content), len(node.Content))
	for i, item := range node.Content {
		d.walk(item, s.Index(i), fmt.Sprintf("%s[%d]", path, i))
	}
	v.Set(s)
}

// dictionary walks a mapping into a map with string keys.
func (d *decoder) dictionary(node *yaml.Node, v reflect.Value, path string) {
	if node.Kind != yaml.MappingNode {
		d.add(node, path, "must be a mapping")
		return
	}
	m := reflect.MakeMapWithSize(v.Type(), len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		keyPath := join(path, key.Value)
		if m.MapIndex(reflect.ValueOf(key.Value).Convert(v.Type().Key())).IsValid() {
			d.add(key, keyPath, "is set twice")
			continue
		}
		elem := reflect.New(v.Type().Elem()).Elem()
		d.walk(value, elem, keyPath)
		m.SetMapIndex(reflect.ValueOf(key.Value).Convert(v.Type().Key()), elem)
	}
	v.Set(m)
}

// scalar decodes a single value; what it should have been is the problem
// when it cannot.
func (d *decoder) scalar(node *yaml.Node, v reflect.Value, path, want string) {
	if node.Kind != yaml.ScalarNode {
		d.add(node, path, "must be "+want)
		return
	}
	if err := node.Decode(v.Addr().Interface()); err != nil {
		d.add(node, path, "must be "+want)
	}
}

func describe(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "a whole number"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "a whole number of zero or more"
	case reflect.Float32, reflect.Float64:
		return "a number"
	default:
		return "a single value"
	}
}

// fieldsOf maps a struct's yaml keys to its field indexes. A field without a
// yaml tag, or tagged "-", is no key.
func fieldsOf(t reflect.Type) map[string]int {
	fields := map[string]int{}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		fields[name] = i
	}
	return fields
}

func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == "!!null"
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
