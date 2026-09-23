package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	methodsPath = "../../spec/methods.json"
	vectorsGlob = "../../vectors/v1/*.json"

	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// vector is the subset of a vectors/v1 file these tests read.
type vector struct {
	Method      *string `json:"method"`
	Requirement string  `json:"requirement"`
	Direction   string  `json:"direction"`
	Kind        string  `json:"kind"`
	Request     *frame  `json:"request"`
	Response    *frame  `json:"response"`
	file        string
}

// malformedParamsRequirement is the requirement of the vectors whose params
// do not decode. A capability may also answer -32602 for params that decode
// but name something unknown, such as an MCP tool (MCP-6).
const malformedParamsRequirement = "WIRE-6"

// frame is a JSON-RPC message of a vector.
type frame struct {
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

func loadVectors(t *testing.T) []vector {
	t.Helper()

	files, err := filepath.Glob(vectorsGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no vectors match %s", vectorsGlob)
	}

	vectors := make([]vector, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var v vector
		if err = json.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		v.file = filepath.Base(file)
		vectors = append(vectors, v)
	}
	return vectors
}

func errorCode(f *frame) int {
	if f == nil || f.Error == nil {
		return 0
	}
	return f.Error.Code
}

func methodIndex(t *testing.T) map[string]Method {
	t.Helper()

	methods, err := Collect()
	if err != nil {
		t.Fatal(err)
	}
	index := make(map[string]Method, len(methods))
	for _, m := range methods {
		index[m.RPCName] = m
	}
	return index
}

func TestMethodsJSONUpToDate(t *testing.T) {
	want, err := Render()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(methodsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale, run `make generate`", methodsPath)
	}
}

// TestVectorMethodsHaveRPCs asserts that every method a vector exercises is an
// rpc of the proto, with the same direction and kind.
func TestVectorMethodsHaveRPCs(t *testing.T) {
	index := methodIndex(t)

	for _, v := range loadVectors(t) {
		if v.Method == nil {
			// Parse and invalid request vectors carry no usable method.
			continue
		}
		m, ok := index[*v.Method]

		if errorCode(v.Response) == codeMethodNotFound {
			// A streaming rpc has no JSON-RPC form, so stdio answers -32601
			// for it like for an unknown method (WIRE-12).
			if ok && !m.Streaming {
				t.Errorf("%s: %s is expected to be unknown but is an rpc", v.file, *v.Method)
			}
			if ok && v.Direction != m.Direction {
				t.Errorf("%s: direction %s, proto says %s", v.file, v.Direction, m.Direction)
			}
			continue
		}
		if m.Streaming {
			t.Errorf("%s: %s is a streaming rpc, a vector can only show stdio refusing it", v.file, *v.Method)
			continue
		}
		if !ok {
			t.Errorf("%s: method %s has no rpc in the proto", v.file, *v.Method)
			continue
		}
		if v.Direction != m.Direction {
			t.Errorf("%s: direction %s, proto says %s", v.file, v.Direction, m.Direction)
		}
		if isNotification := v.Kind == "notification"; isNotification != m.Notification {
			t.Errorf("%s: kind %s, proto notification=%v", v.file, v.Kind, m.Notification)
		}
	}
}

// TestVectorPayloadsDecode asserts that params and results of the vectors are
// the protobuf JSON mapping of the rpc messages, and that re-encoding them
// keeps every non-default value. A WIRE-6 vector that expects -32602 carries
// params that must not decode.
func TestVectorPayloadsDecode(t *testing.T) {
	index := methodIndex(t)

	for _, v := range loadVectors(t) {
		if v.Method == nil {
			continue
		}
		m, ok := index[*v.Method]
		if !ok {
			continue
		}

		if v.Request != nil && len(v.Request.Params) > 0 {
			err := roundTrip(m.Request, v.Request.Params)
			if errorCode(v.Response) == codeInvalidParams && v.Requirement == malformedParamsRequirement {
				if err == nil {
					t.Errorf("%s: params decoded although the vector expects invalid params", v.file)
				}
			} else if err != nil {
				t.Errorf("%s: params: %v", v.file, err)
			}
		}

		if v.Response != nil && len(v.Response.Result) > 0 {
			if err := roundTrip(m.Response, v.Response.Result); err != nil {
				t.Errorf("%s: result: %v", v.file, err)
			}
		}
	}
}

// roundTrip decodes data into the named message strictly and checks that the
// canonical encoding carries the same values.
func roundTrip(name string, data json.RawMessage) error {
	mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(name))
	if err != nil {
		return err
	}

	msg := mt.New().Interface()
	if err = protojson.Unmarshal(data, msg); err != nil {
		return err
	}
	encoded, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return err
	}

	var want, got any
	if err = json.Unmarshal(data, &want); err != nil {
		return err
	}
	if err = json.Unmarshal(encoded, &got); err != nil {
		return err
	}
	if w, g := normalize(want), normalize(got); !reflect.DeepEqual(w, g) {
		return &mismatchError{want: w, got: g}
	}
	return nil
}

type mismatchError struct {
	want, got any
}

func (e *mismatchError) Error() string {
	want, _ := json.Marshal(e.want)
	got, _ := json.Marshal(e.got)
	return "re-encoded payload differs\nwant " + string(want) + "\ngot  " + string(got)
}

// normalize drops members holding a default value, since the canonical
// encoding omits them.
func normalize(v any) any {
	switch value := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, item := range value {
			if n := normalize(item); n != nil {
				out[k] = n
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		if len(value) == 0 {
			return nil
		}
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = normalize(item)
		}
		return out
	case string:
		if value == "" {
			return nil
		}
	case bool:
		if !value {
			return nil
		}
	case float64:
		if value == 0 {
			return nil
		}
	}
	return v
}

// TestStreamingRPCs asserts that the streaming flag marks exactly the client
// streaming rpcs, which are never notifications.
func TestStreamingRPCs(t *testing.T) {
	var streaming []string
	for _, m := range methodIndex(t) {
		if !m.Streaming {
			continue
		}
		streaming = append(streaming, m.RPCName)
		if m.Notification {
			t.Errorf("%s: a streaming rpc cannot be a notification", m.RPCName)
		}
		md, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(m.Service))
		if err != nil {
			t.Fatal(err)
		}
		rpc := md.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(m.Method))
		if rpc == nil || !rpc.IsStreamingClient() || rpc.IsStreamingServer() {
			t.Errorf("%s: not a client streaming rpc", m.RPCName)
		}
	}
	if len(streaming) != 1 || streaming[0] != "log.push" {
		t.Errorf("streaming rpcs = %v, want [log.push]", streaming)
	}
}
