// Command methods writes spec/methods.json, the table that maps every
// JSON-RPC method name of the plugin contract onto its proto rpc.
//
// It reads the file descriptors compiled into the generated Go package, so
// run it after `buf generate`:
//
//	go run ./methods -out ../spec/methods.json
//	go run ./methods -out ../spec/methods.json -check
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	pluginv1 "github.com/nginxui/plugin-spec/gen/go/nginxui/plugin/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Direction values used by spec/methods.json and vectors/v1.
const (
	HostToPlugin = "host_to_plugin"
	PluginToHost = "plugin_to_host"
)

// serviceDirections says who calls each service. nginx-ui serves Host and the
// plugin process serves everything else. A new service must be added here.
var serviceDirections = map[protoreflect.Name]string{
	"Plugin":    HostToPlugin,
	"DNS01":     HostToPlugin,
	"HTTP":      HostToPlugin,
	"Notify":    HostToPlugin,
	"Probe":     HostToPlugin,
	"MCP":       HostToPlugin,
	"Storage":   HostToPlugin,
	"Deploy":    HostToPlugin,
	"Blocklist": HostToPlugin,
	"Discovery": HostToPlugin,
	"LogSink":   HostToPlugin,
	"Events":    HostToPlugin,
	"Host":      PluginToHost,
}

// Method is one entry of spec/methods.json.
type Method struct {
	RPCName      string `json:"rpc_name"`
	Service      string `json:"service"`
	Method       string `json:"method"`
	FullMethod   string `json:"full_method"`
	Request      string `json:"request"`
	Response     string `json:"response"`
	Notification bool   `json:"notification"`
	Direction    string `json:"direction"`
	// Streaming marks a client streaming rpc, which travels on gRPC only
	// (WIRE-12). It is omitted for the unary rpcs.
	Streaming bool `json:"streaming,omitempty"`
}

// Package is the proto package of the contract.
var Package = pluginv1.File_nginxui_plugin_v1_options_proto.Package()

// Collect returns every rpc of the contract package, ordered by file path and
// then by declaration order.
func Collect() ([]Method, error) {
	var files []protoreflect.FileDescriptor
	protoregistry.GlobalFiles.RangeFilesByPackage(Package, func(fd protoreflect.FileDescriptor) bool {
		files = append(files, fd)
		return true
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })

	var methods []Method
	seen := map[string]protoreflect.FullName{}
	for _, file := range files {
		services := file.Services()
		for i := range services.Len() {
			sd := services.Get(i)
			direction, ok := serviceDirections[sd.Name()]
			if !ok {
				return nil, fmt.Errorf("service %s has no direction, add it to serviceDirections", sd.FullName())
			}

			rpcs := sd.Methods()
			for j := range rpcs.Len() {
				md := rpcs.Get(j)
				if md.IsStreamingServer() {
					return nil, fmt.Errorf("%s: only client streaming rpcs are supported", md.FullName())
				}
				streaming := proto.GetExtension(md.Options(), pluginv1.E_Streaming).(bool)
				if streaming != md.IsStreamingClient() {
					return nil, fmt.Errorf("%s: a streamed request and the streaming option go together", md.FullName())
				}

				rpcName := proto.GetExtension(md.Options(), pluginv1.E_RpcName).(string)
				if rpcName == "" {
					return nil, fmt.Errorf("%s: missing the rpc_name option", md.FullName())
				}
				if prev, dup := seen[rpcName]; dup {
					return nil, fmt.Errorf("%s: rpc_name %q is already used by %s", md.FullName(), rpcName, prev)
				}
				seen[rpcName] = md.FullName()

				notification := proto.GetExtension(md.Options(), pluginv1.E_Notification).(bool)
				if notification && md.Output().Fields().Len() != 0 {
					return nil, fmt.Errorf("%s: a notification must return an empty message", md.FullName())
				}
				if notification && streaming {
					return nil, fmt.Errorf("%s: a streaming rpc cannot be a notification", md.FullName())
				}

				methods = append(methods, Method{
					RPCName:      rpcName,
					Service:      string(sd.FullName()),
					Method:       string(md.Name()),
					FullMethod:   fmt.Sprintf("/%s/%s", sd.FullName(), md.Name()),
					Request:      string(md.Input().FullName()),
					Response:     string(md.Output().FullName()),
					Notification: notification,
					Direction:    direction,
					Streaming:    streaming,
				})
			}
		}
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no rpc found in package %s", Package)
	}
	return methods, nil
}

// Render returns the content of spec/methods.json.
func Render() ([]byte, error) {
	methods, err := Collect()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err = enc.Encode(methods); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func main() {
	out := flag.String("out", "../spec/methods.json", "path of the methods table")
	check := flag.Bool("check", false, "fail when the file on disk differs instead of writing it")
	flag.Parse()

	if err := run(*out, *check); err != nil {
		fmt.Fprintln(os.Stderr, "methods:", err)
		os.Exit(1)
	}
}

func run(out string, check bool) error {
	want, err := Render()
	if err != nil {
		return err
	}

	if !check {
		return os.WriteFile(out, want, 0o644)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s is stale, run `make generate`", out)
	}
	return nil
}
