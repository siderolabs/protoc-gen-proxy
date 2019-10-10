/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proxy

import (
	"fmt"
	"strings"

	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// generatedCodeVersion indicates a version of the generated code.
// It is incremented whenever an incompatibility between the generated code and
// the grpc package is introduced; the generated code references
// a constant, grpc.SupportPackageIsVersionN (where N is generatedCodeVersion).
const generatedCodeVersion = 4

// Paths for packages used by code generated in this file,
// relative to the import_prefix of the generator.Generator.
const (
	contextPkgPath = "context"
	grpcPkgPath    = "google.golang.org/grpc"
	codePkgPath    = "google.golang.org/grpc/codes"
	statusPkgPath  = "google.golang.org/grpc/status"
)

func init() {
	generator.RegisterPlugin(new(proxy))
}

// grpc is an implementation of the Go protocol buffer compiler's
// plugin architecture.  It generates bindings for gRPC support.
type proxy struct {
	gen *generator.Generator
}

// Name returns the name of this plugin, "grpc".
func (g *proxy) Name() string {
	return "proxy"
}

// The names for packages imported in the generated code.
// They may vary from the final path component of the import path
// if the name is used by other packages.
var (
	contextPkg string
	grpcPkg    string
)

// Init initializes the plugin.
func (g *proxy) Init(gen *generator.Generator) {
	g.gen = gen
}

// Given a type name defined in a .proto, return its object.
// Also record that we're using it, to guarantee the associated import.
func (g *proxy) objectNamed(name string) generator.Object {
	g.gen.RecordTypeUse(name)
	return g.gen.ObjectNamed(name)
}

// Given a type name defined in a .proto, return its name as we will print it.
func (g *proxy) typeName(str string) string {
	return g.gen.TypeName(g.objectNamed(str))
}

// P forwards to g.gen.P.
func (g *proxy) P(args ...interface{}) { g.gen.P(args...) }

// GenerateImports generates the import declaration for this file.
func (g *proxy) GenerateImports(file *generator.FileDescriptor) {
}

func (g *proxy) Generate(file *generator.FileDescriptor) {
	g.gen.AddImport("strings")
	g.gen.AddImport("sync")
	g.gen.AddImport("google.golang.org/grpc/metadata")
	g.gen.AddImport("google.golang.org/grpc/credentials")
	g.gen.AddImport("github.com/hashicorp/go-multierror")
	g.gen.AddImport("github.com/talos-systems/talos/pkg/grpc/tls")

	for _, service := range file.FileDescriptorProto.Service {
		serviceName := generator.CamelCase(service.GetName())
		g.generateServiceFuncType(serviceName)
		g.P("")
		g.generateProxyClientStruct(serviceName)
		g.P("")
		g.generateProxyRunner(serviceName)
		g.P("")
		g.generateProxyStruct(serviceName)
		g.P("")
		g.generateProxyInterceptor(serviceName, file.GetPackage())
		g.P("")
		g.generateProxyRouter(serviceName, file.GetPackage(), service.Method)
		for _, method := range service.Method {
			// No support for streaming stuff yet
			if method.GetServerStreaming() || method.GetClientStreaming() {
				continue
			}
			g.P("")
			g.generateServiceFunc(serviceName, method)
		}
	}
}

// generateServiceFuncType is a function with a specific signature. This function
// gets passed through the 'runner' func to perform the actual client call.
func (g *proxy) generateServiceFuncType(serviceName string) {
	var args strings.Builder
	args.WriteString("(")
	args.WriteString("*proxy" + serviceName + "Client, ")
	args.WriteString("interface{}, ")
	args.WriteString("*sync.WaitGroup, ")
	args.WriteString("chan proto.Message, ")
	args.WriteString("chan error")
	args.WriteString(")")
	g.P("type runnerfn func" + args.String())
}

// generateProxyClientStruct holds the client connection and additional metadata
// associated with each grpc ( client ) connection that the proxy creates. This
// should only exist for the duration of the request.
func (g *proxy) generateProxyClientStruct(serviceName string) {
	g.P("type proxy" + serviceName + "Client struct {")
	g.P("Conn " + serviceName + "Client")
	g.P("Context context.Context")
	g.P("Target string")
	g.P("DialOpts []grpc.DialOption")
	g.P("}")
}

// generateProxyStruct is the public struct exposed for use by importers. It
// contains a tls provider to manage the TLS cert rotation/renewal. This also
// generates the constructor for the struct.
func (g *proxy) generateProxyStruct(serviceName string) {
	tName := generator.CamelCase(serviceName + "_proxy")
	g.P("type " + tName + " struct {")
	g.P("Provider tls.CertificateProvider")
	g.P("}")

	g.P("")

	var args strings.Builder
	args.WriteString("(")
	args.WriteString("provider tls.CertificateProvider")
	args.WriteString(")")
	g.P("func New" + tName + args.String() + " *" + tName + "{")
	g.P("return &" + tName + "{Provider: provider}")
	g.P("}")
}

// generateProxyInterceptor is a method of the proxy struct that satisfies the
// grpc.UnaryInterceptor interface. This allows us to make use of the tls
// information from the provider to include it with each subsequent request
// from the proxy. This is also where we handle some of the routing decisions,
// namely being able to filter on the supported service and handling the
// 'proxyfrom' metadata field to prevent infinite loops.
func (g *proxy) generateProxyInterceptor(serviceName string, pkgName string) {
	tName := generator.CamelCase(serviceName + "_proxy")
	fullServName := serviceName
	if pkgName != "" {
		fullServName = pkgName + "." + fullServName
	}
	g.P("func (p *" + tName + ") UnaryInterceptor() grpc.UnaryServerInterceptor {")
	g.P("return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {")
	// Artifically limit scope to OS api
	g.P("pkg := strings.Split(info.FullMethod, \"/\")[1]")
	g.P("if pkg != \"" + fullServName + "\" {")
	g.P("return handler(ctx, req)")
	g.P("}")
	g.P("md, _ := metadata.FromIncomingContext(ctx)")
	g.P("if _, ok := md[\"proxyfrom\"]; ok {")
	g.P("return handler(ctx, req)")
	g.P("}")
	g.P("ca, err := p.Provider.GetCA()")
	g.P("if err != nil {")
	g.P("	return nil, err")
	g.P("}")
	g.P("certs, err := p.Provider.GetCertificate(nil)")
	g.P("if err != nil {")
	g.P("  return nil, err")
	g.P("}")
	g.P("tlsConfig, err := tls.New(")
	g.P("  tls.WithClientAuthType(tls.Mutual),")
	g.P("  tls.WithCACertPEM(ca),")
	g.P("  tls.WithKeypair(*certs),")
	g.P(")")
	g.P("return p.Proxy(ctx, info.FullMethod, credentials.NewTLS(tlsConfig), req)")
	g.P("}")
	g.P("}")
}

// generateServiceFunc is a function generated for each service defined in the
// proto file. The function signature satisfies the runnerfn type.
func (g *proxy) generateServiceFunc(serviceName string, method *pb.MethodDescriptorProto) {
	var args strings.Builder
	args.WriteString("(")
	args.WriteString("client *proxy" + serviceName + "Client, ")
	args.WriteString("in interface{}, ")
	args.WriteString("wg *sync.WaitGroup, ")
	args.WriteString("respCh chan proto.Message, ")
	args.WriteString("errCh chan error")
	args.WriteString(")")

	g.P("func proxy" + generator.CamelCase(method.GetName()) + args.String() + "{")
	g.P("defer wg.Done()")
	g.P("resp, err := client.Conn." + method.GetName() + "(client.Context, in.(*" + g.typeName(method.GetInputType()) + "))")
	g.P("if err != nil {")
	g.P("errCh<-err")
	g.P("return")
	g.P("}")
	// TODO: See if we can better abstract this
	g.P("resp.Response[0].Metadata = &NodeMetadata{Hostname: client.Target}")
	g.P("respCh<-resp")
	g.P("}")
}

// generateProxyRunner is the function that handles the client calls and response
// aggregation.
func (g *proxy) generateProxyRunner(serviceName string) {
	var args strings.Builder
	args.WriteString("(")
	args.WriteString("clients []*proxy" + serviceName + "Client, ")
	args.WriteString("in interface{}, ")
	args.WriteString("runner runnerfn")
	args.WriteString(")")

	var returns strings.Builder
	returns.WriteString("(")
	returns.WriteString("[]proto.Message, ")
	returns.WriteString("error")
	returns.WriteString(")")

	g.P("func proxy" + serviceName + "Runner" + args.String() + returns.String() + "{")
	g.P("var (")
	g.P("errors *go_multierror.Error")
	g.P("wg sync.WaitGroup")
	g.P(")")

	g.P("respCh := make(chan proto.Message, len(clients))")
	g.P("errCh := make(chan error, len(clients))")
	g.P("wg.Add(len(clients))")

	g.P("for _, client := range clients {")
	g.P("go runner(client, in, &wg, respCh, errCh)")
	g.P("}")

	g.P("wg.Wait()")
	g.P("close(respCh)")
	g.P("close(errCh)")
	g.P("")

	g.P("var response []proto.Message")
	g.P("for resp := range respCh {")
	g.P("response = append(response, resp)")
	g.P("}")

	g.P("for err := range errCh {")
	g.P("errors = go_multierror.Append(errors, err)")
	g.P("}")

	g.P("return response, errors.ErrorOrNil()")
	g.P("}")
}

// generateProxyRouter creates the routing part of the proxy. That is it
// enables us to map the incoming grpc method to the function/client call
// so we can properly call the proper rpc endpoint.
func (g *proxy) generateProxyRouter(serviceName string, pkgName string, methods []*pb.MethodDescriptorProto) {
	// Leaving this in as left overs from grpc plugin, but not
	// sure it really makes sense to keep it like this versus
	// just calling the addimport call in Generate()
	contextPkg = string(g.gen.AddImport(contextPkgPath))
	grpcPkg = string(g.gen.AddImport(grpcPkgPath))

	var args strings.Builder
	args.WriteString("(")
	args.WriteString("ctx " + contextPkg + ".Context, ")
	args.WriteString("method string, ")
	args.WriteString("creds credentials.TransportCredentials, ")
	args.WriteString("in interface{}, ")
	args.WriteString("opts ..." + grpcPkg + ".CallOption")
	args.WriteString(")")

	var returns strings.Builder
	returns.WriteString("(")
	returns.WriteString("proto.Message, ")
	returns.WriteString("error")
	returns.WriteString(")")

	g.P("func (p *" + generator.CamelCase(serviceName+"_proxy") + ") Proxy " + args.String() + returns.String() + "{")
	g.P("var (")
	g.P("err error")
	g.P("errors *go_multierror.Error")
	g.P("msgs []proto.Message")
	g.P("ok bool")
	g.P("response proto.Message")
	g.P("targets []string")
	g.P(")")

	// Parse targets from incoming metadata/context
	g.P("md, _ := metadata.FromIncomingContext(ctx)")
	g.P("// default to target node specified in config or on cli")
	g.P("if targets, ok = md[\"targets\"]; !ok {")
	g.P("targets = md[\":authority\"]")
	g.P("}")

	// Set up client connections
	g.P("proxyMd := metadata.New(make(map[string]string))")
	g.P("proxyMd.Set(\"proxyfrom\", md[\":authority\"]...)")
	g.P("")
	g.P("clients := []*proxy" + serviceName + "Client{}")
	g.P("for _, target := range targets {")
	g.P("c := &proxy" + serviceName + "Client{")
	// TODO change the context to be more useful ( ex cancelable )
	g.P("Context: metadata.NewOutgoingContext(context.Background(), proxyMd),")
	g.P("Target:  target,")
	g.P("}")
	g.P("overrideCreds := creds")
	// Explicitly set OSD port
	// TODO: i think we potentially leak a client here,
	// we should close the request // cancel the context if it errors
	g.P("conn, err := grpc.Dial(fmt.Sprintf(\"%s:%d\", c.Target, 50000), grpc.WithTransportCredentials(overrideCreds))")
	g.P("if err != nil {")
	// TODO: probably worth wrapping err to add some context about the target
	g.P("errors = go_multierror.Append(errors, err)")
	g.P("continue")
	g.P("}")
	g.P("c.Conn = New" + serviceName + "Client(conn)")
	g.P("clients = append(clients, c)")
	g.P("}")

	// Handle routes
	g.P("switch method {")
	for _, method := range methods {
		// No support for streaming stuff yet
		if method.GetServerStreaming() || method.GetClientStreaming() {
			continue
		}
		fullServName := serviceName
		if pkgName != "" {
			fullServName = pkgName + "." + fullServName
		}
		sname := fmt.Sprintf("/%s/%s", fullServName, method.GetName())
		g.P("case \"" + sname + "\":")
		var fnArgs strings.Builder
		fnArgs.WriteString("(")
		fnArgs.WriteString("clients, ")
		fnArgs.WriteString("in, ")
		fnArgs.WriteString("proxy" + generator.CamelCase(method.GetName()))
		fnArgs.WriteString(")")
		g.P("resp := &" + g.typeName(method.GetOutputType()) + "{}")
		g.P("msgs, err = proxy" + serviceName + "Runner" + fnArgs.String())
		g.P("for _, msg := range msgs {")
		g.P("resp.Response = append(resp.Response, msg.(*" + g.typeName(method.GetOutputType()) + ").Response[0])")
		g.P("}")
		g.P("response = resp")
	}
	g.P("}")
	g.P("")
	g.P("if err != nil {")
	g.P("errors = go_multierror.Append(errors, err)")
	g.P("}")
	g.P("return response, errors.ErrorOrNil()")
	g.P("}")
}
