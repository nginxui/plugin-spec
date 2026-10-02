package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	pluginv1 "github.com/nginxui/plugin-spec/gen/go/nginxui/plugin/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	schemaPath   = "../../schema/plugin.schema.json"
	examplesGlob = "../../examples/*/plugin.json"
)

type node = map[string]any

// walker compares a JSON Schema object with a proto message, recursing into
// nested messages.
type walker struct {
	t       *testing.T
	defs    node
	visited map[string]bool
}

// TestSchemaMatchesManifestProto asserts that every object of the schema
// reachable from the root has exactly the properties of the proto message at
// the same position, with a compatible JSON type.
func TestSchemaMatchesManifestProto(t *testing.T) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var root node
	if err = json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}

	w := &walker{t: t, defs: root["$defs"].(node), visited: map[string]bool{}}
	w.check((&pluginv1.Manifest{}).ProtoReflect().Descriptor(), root, "$")

	// An object definition nobody reaches would escape the comparison.
	for name, def := range w.defs {
		if _, isObject := def.(node)["properties"]; isObject && !w.visited[name] {
			t.Errorf("$defs/%s is not reachable from the manifest root", name)
		}
	}
}

func (w *walker) check(md protoreflect.MessageDescriptor, n node, path string) {
	props, _ := n["properties"].(node)
	if props == nil {
		w.t.Errorf("%s: schema has no properties for %s", path, md.FullName())
		return
	}

	fields := md.Fields()
	var protoNames, schemaNames []string
	for i := range fields.Len() {
		protoNames = append(protoNames, string(fields.Get(i).Name()))
	}
	for name := range props {
		schemaNames = append(schemaNames, name)
	}
	for _, name := range missing(protoNames, schemaNames) {
		w.t.Errorf("%s: field %s.%s is not a schema property", path, md.FullName(), name)
	}
	for _, name := range missing(schemaNames, protoNames) {
		w.t.Errorf("%s: schema property %s is not a field of %s", path, name, md.FullName())
	}

	for i := range fields.Len() {
		fd := fields.Get(i)
		prop, ok := props[string(fd.Name())].(node)
		if !ok {
			continue
		}
		w.checkField(fd, w.resolve(prop), path+"."+string(fd.Name()))
	}
}

func (w *walker) checkField(fd protoreflect.FieldDescriptor, n node, path string) {
	switch {
	case fd.IsMap():
		w.expectType(n, "object", path)
		if values, ok := n["additionalProperties"].(node); ok {
			w.checkSingular(fd.MapValue(), w.resolve(values), path+"{}")
		}
	case fd.IsList():
		w.expectType(n, "array", path)
		if items, ok := n["items"].(node); ok {
			w.checkSingular(fd, w.resolve(items), path+"[]")
		}
	default:
		w.checkSingular(fd, n, path)
	}
}

func (w *walker) checkSingular(fd protoreflect.FieldDescriptor, n node, path string) {
	switch fd.Kind() {
	case protoreflect.StringKind, protoreflect.BytesKind:
		w.expectType(n, "string", path)
	case protoreflect.BoolKind:
		w.expectType(n, "boolean", path)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		w.expectType(n, "integer", path)
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		w.expectType(n, "number", path)
	case protoreflect.MessageKind:
		switch fd.Message().FullName() {
		case "google.protobuf.Value":
			// Any JSON value, nothing to compare.
		case "google.protobuf.Struct":
			w.expectType(n, "object", path)
		case "google.protobuf.ListValue":
			w.expectType(n, "array", path)
		default:
			w.expectType(n, "object", path)
			w.check(fd.Message(), n, path)
		}
	default:
		w.t.Errorf("%s: proto kind %s has no JSON Schema mapping here", path, fd.Kind())
	}
}

func (w *walker) expectType(n node, want, path string) {
	got, ok := n["type"].(string)
	if !ok {
		return
	}
	if got != want {
		w.t.Errorf("%s: schema type %s, proto maps to %s", path, got, want)
	}
}

// resolve follows $ref and picks the non-null branch of a nullable oneOf.
func (w *walker) resolve(n node) node {
	for {
		if ref, ok := n["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/$defs/")
			def, found := w.defs[name].(node)
			if !found {
				w.t.Fatalf("unresolved $ref %s", ref)
			}
			w.visited[name] = true
			n = def
			continue
		}

		branches, ok := n["oneOf"].([]any)
		if !ok || n["type"] != nil || n["properties"] != nil {
			return n
		}
		var picked node
		for _, branch := range branches {
			if b := branch.(node); b["type"] != "null" {
				if picked != nil {
					w.t.Fatalf("oneOf with more than one non-null branch: %v", n)
				}
				picked = b
			}
		}
		n = picked
	}
}

// missing returns the names of a that are not in b, sorted.
func missing(a, b []string) []string {
	var out []string
	for _, name := range a {
		if !slices.Contains(b, name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// TestExampleManifestsDecode asserts that every example plugin.json is the
// protobuf JSON mapping of Manifest.
func TestExampleManifestsDecode(t *testing.T) {
	files, err := filepath.Glob(examplesGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no manifest matches %s", examplesGlob)
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err = protojson.Unmarshal(data, &pluginv1.Manifest{}); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
}

// TestCatalogSnapshotMembersAreManifestMembers asserts that every member of
// the manifest snapshot in catalog.schema.json refers to the plugin.json
// member of the same name.
func TestCatalogSnapshotMembersAreManifestMembers(t *testing.T) {
	var manifest, catalog node
	for path, target := range map[string]*node{schemaPath: &manifest, "../../schema/catalog.schema.json": &catalog} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}

	members := manifest["properties"].(node)
	snapshot := catalog["$defs"].(node)["manifestSnapshot"].(node)
	for name, schema := range snapshot["properties"].(node) {
		if _, ok := members[name]; !ok {
			t.Errorf("snapshot member %q is not a plugin.json member", name)
		}
		if ref := schema.(node)["$ref"]; ref != "plugin.schema.json#/properties/"+name {
			t.Errorf("snapshot member %q refers to %v", name, ref)
		}
	}
	for _, name := range snapshot["required"].([]any) {
		if !slices.Contains(manifest["required"].([]any), name) {
			t.Errorf("snapshot requires %v, which plugin.json does not", name)
		}
	}
}

// TestCatalogProvidesRefersToManifestDefinitions asserts that the members of
// a provided dns01 provider refer to definitions plugin.schema.json has.
func TestCatalogProvidesRefersToManifestDefinitions(t *testing.T) {
	var manifest, catalog node
	for path, target := range map[string]*node{schemaPath: &manifest, "../../schema/catalog.schema.json": &catalog} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}

	defs := catalog["$defs"].(node)
	members := map[string]any{"since of dns01": defs["providesDns01"].(node)["properties"].(node)["since"]}
	for name, schema := range defs["providedDns01"].(node)["properties"].(node) {
		members[name] = schema
	}
	for name, schema := range members {
		ref, _ := schema.(node)["$ref"].(string)
		pointer, ok := strings.CutPrefix(ref, "plugin.schema.json#/")
		if !ok {
			t.Errorf("provided dns01 member %q refers to %q, not into plugin.schema.json", name, ref)
			continue
		}
		var at any = manifest
		for _, part := range strings.Split(pointer, "/") {
			object, isObject := at.(node)
			if !isObject {
				at = nil
				break
			}
			at = object[part]
		}
		if at == nil {
			t.Errorf("provided dns01 member %q refers to %q, which plugin.schema.json lacks", name, ref)
		}
	}
}
