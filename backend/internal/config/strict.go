package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// decodeStrict decodes a YAML document into out, a pointer to a struct whose
// fields carry yaml tags, and collects every problem it finds instead of
// stopping at the first: a key the struct does not name, a key set twice, a
// mapping, list or single value where another is expected, a value its field
// cannot hold. Each problem names the line and the key's path. The struct is
// the schema: a field is a known key, its type what the key holds. The
// decoder it returns also places every path it read on its line, and knows
// the paths that failed, for the problems the validation finds after it.
func decodeStrict(data []byte, out any) *decoder {
	d := &decoder{lines: lines{}, failed: map[string]bool{}}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		d.problems = append(d.problems, problem{message: strings.TrimPrefix(err.Error(), "yaml: ")})
		d.failed[""] = true
		return d
	}
	if len(doc.Content) == 0 {
		return d // an empty file sets nothing
	}
	d.walk(doc.Content[0], reflect.ValueOf(out).Elem(), "")
	return d
}

type decoder struct {
	problems []problem
	lines    lines
	// failed are the paths whose value could not be decoded: the validation
	// says nothing more about them or what they hold.
	failed map[string]bool
}

// problem is one thing wrong with the file, at a line (0: not known) and a
// key's path ("": the message says where).
type problem struct {
	line    int
	path    string
	message string
}

func (p problem) String() string {
	switch {
	case p.path == "":
		return p.message
	case p.line > 0:
		return fmt.Sprintf("line %d: %s: %s", p.line, p.path, p.message)
	default:
		return p.path + ": " + p.message
	}
}

// messages are the problems as they are reported.
func (d *decoder) messages() []string {
	out := make([]string, 0, len(d.problems))
	for _, p := range d.problems {
		out = append(out, p.String())
	}
	return out
}

// lines maps a key's path, such as apps[0].apps[1].url, to the line of the
// key — of the item for a list's item.
type lines map[string]int

// of is the line of path, else of the nearest enclosing path the file
// sets — the mapping a missing key belongs in — else 0: the file sets
// nothing on the way.
func (l lines) of(path string) int {
	for path != "" {
		if n, ok := l[path]; ok {
			return n
		}
		path = parent(path)
	}
	return l[""]
}

// parent is the path that encloses path: a.b[2].c → a.b[2] → a.b → a → "".
func parent(path string) string {
	i := strings.LastIndexAny(path, ".[")
	if i < 0 {
		return ""
	}
	return path[:i]
}

func (d *decoder) add(node *yaml.Node, path, message string) {
	d.failed[path] = true
	if path == "" {
		path = "the document"
	}
	d.problems = append(d.problems, problem{line: node.Line, path: path, message: message})
}

var durationType = reflect.TypeFor[time.Duration]()

func (d *decoder) walk(node *yaml.Node, v reflect.Value, path string) {
	if node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	if _, keyed := d.lines[path]; !keyed {
		d.lines[path] = node.Line // a list item, or the document
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
		if !seen[key.Value] {
			d.lines[keyPath] = key.Line
		}
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
		keyPath := mapKey(path, key.Value)
		if m.MapIndex(reflect.ValueOf(key.Value).Convert(v.Type().Key())).IsValid() {
			d.add(key, keyPath, "is set twice")
			continue
		}
		d.lines[keyPath] = key.Line
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

// mapKey is the path of a map's key: path.key, or path["key"] for a key
// that holds a '.', such as the label key app.kubernetes.io/name, so the
// path still reads as one key.
func mapKey(path, key string) string {
	if strings.ContainsAny(key, ".[]") {
		return fmt.Sprintf("%s[%q]", path, key)
	}
	return join(path, key)
}
