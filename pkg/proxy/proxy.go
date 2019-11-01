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

// generateProxyClientStruct holds the client connection and additional metadata
// associated with each grpc ( client ) connection that the proxy creates. This
// should only exist for the duration of the request.
func (g *proxy) generateProxyClientStruct(serviceName, pkgName string) {
	g.P(g.ProxyFns, "type proxy"+serviceName+"Client struct {")
	g.P(g.ProxyFns, "Conn "+pkgName+"."+serviceName+"Client")
	g.P(g.ProxyFns, "Context context.Context")
	g.P(g.ProxyFns, "Target string")
	g.P(g.ProxyFns, "DialOpts []grpc.DialOption")
	g.P(g.ProxyFns, "}")
	g.P(g.ProxyFns, "")
}

// generateProxyStruct is the public struct exposed for use by importers. It
// contains a tls provider to manage the TLS cert rotation/renewal. This also
// generates the constructor for the struct.
func (g *proxy) generateProxyStruct(serviceName string) {
	tName := generator.CamelCase(serviceName + "_proxy")
	g.gen.P("type " + tName + " struct {")
	g.gen.P("Provider tls.CertificateProvider")
	g.gen.P("}")
	g.gen.P("")

	var args strings.Builder
	args.WriteString("(")
	args.WriteString("provider tls.CertificateProvider")
	args.WriteString(")")
	g.gen.P("func New" + tName + args.String() + " *" + tName + "{")
	g.gen.P("return &" + tName + "{")
	g.gen.P("Provider: provider,")
	g.gen.P("}")
	g.gen.P("}")
	g.gen.P("")
}

// generateProxyRouter creates the routing part of the proxy. That is it
// enables us to map the incoming grpc method to the function/client call
// so we can properly call the proper rpc endpoint.
func (g *proxy) generateProxyRouter(serviceName string) {
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

	g.gen.P("func (p *" + generator.CamelCase(serviceName+"_proxy") + ") Proxy " + args.String() + returns.String() + "{")
	g.gen.P("var (")
	g.gen.P("err error")
	g.gen.P("errors *go_multierror.Error")
	g.gen.P("msgs []proto.Message")
	g.gen.P("ok bool")
	g.gen.P("response proto.Message")
	g.gen.P("targets []string")
	g.gen.P(")")

	// Parse targets from incoming metadata/context
	g.gen.P("md, _ := metadata.FromIncomingContext(ctx)")
	g.gen.P("// default to target node specified in config or on cli")
	g.gen.P("if targets, ok = md[\"targets\"]; !ok {")
	g.gen.P("targets = md[\":authority\"]")
	g.gen.P("}")

	// Set up client connections
	g.gen.P("proxyMd := metadata.New(make(map[string]string))")
	g.gen.P("proxyMd.Set(\"proxyfrom\", md[\":authority\"]...)")
	g.gen.P("")

	// Handle routes
	g.gen.P("switch method {")
	g.gen.P(g.ProxySwitch.String())
	g.gen.P("}")
	g.gen.P("")
	g.gen.P("if err != nil {")
	g.gen.P("errors = go_multierror.Append(errors, err)")
	g.gen.P("}")
	g.gen.P("return response, errors.ErrorOrNil()")
	g.gen.P("}")
}

func (g *proxy) generateSwitchStatement(serviceName, pkgName string, methods []*pb.MethodDescriptorProto) {
	for _, method := range methods {
		// No support for streaming stuff yet
		if method.GetServerStreaming() || method.GetClientStreaming() {
			continue
		}
		// skip support for deprecated methods
		if method.GetOptions().GetDeprecated() {
			continue
		}
		fullServName := serviceName
		if pkgName != "" {
			fullServName = pkgName + "." + fullServName
		}
		sname := fmt.Sprintf("/%s/%s", fullServName, method.GetName())
		g.P(g.ProxySwitch, "case \""+sname+"\":")
		g.P(g.ProxySwitch, "// Initialize target clients")
		g.P(g.ProxySwitch, "clients, err := create"+serviceName+"Client(targets, creds, proxyMd)")
		g.P(g.ProxySwitch, "if err != nil {")
		g.P(g.ProxySwitch, "break")
		g.P(g.ProxySwitch, "}")

		var fnArgs strings.Builder
		fnArgs.WriteString("(")
		fnArgs.WriteString("clients, ")
		fnArgs.WriteString("in, ")
		fnArgs.WriteString("proxy" + generator.CamelCase(method.GetName()))
		fnArgs.WriteString(")")

		g.P(g.ProxySwitch, "resp := &"+g.typeName(method.GetOutputType())+"{}")
		g.P(g.ProxySwitch, "msgs, err = proxy"+serviceName+"Runner"+fnArgs.String())
		g.P(g.ProxySwitch, "for _, msg := range msgs {")
		g.P(g.ProxySwitch, "resp.Response = append(resp.Response, msg.(*"+g.typeName(method.GetOutputType())+").Response[0])")
		g.P(g.ProxySwitch, "}")
		g.P(g.ProxySwitch, "response = resp")
	}
}

func (g *proxy) generateStreamCopyHelper() {
	g.gen.P("func copyClientServer(msg interface{}, client grpc.ClientStream, srv grpc.ServerStream) error {")
	g.gen.P("	for {")
	g.gen.P("		err := client.RecvMsg(msg)")
	g.gen.P("		if err == io.EOF {")
	g.gen.P("			break")
	g.gen.P("		}")
	g.gen.P("")
	g.gen.P("		if err != nil {")
	g.gen.P("			return err")
	g.gen.P("		}")
	g.gen.P("")
	g.gen.P("		err = srv.SendMsg(msg)")
	g.gen.P("		if err != nil {")
	g.gen.P("			return err")
	g.gen.P("		}")
	g.gen.P("	}")
	g.gen.P("")
	g.gen.P("	return nil")
	g.gen.P("}")
	g.gen.P("")
}

func (g *proxy) generateNoProxyDialer() {
	g.gen.P("func noProxyDialer(ctx context.Context, addr string) (net.Conn, error) {")
	g.gen.P("dst, err := net.ResolveUnixAddr(\"unix\", addr)")
	g.gen.P("if err != nil {")
	g.gen.P("  return nil, err")
	g.gen.P("}")
	g.gen.P("return net.DialUnix(\"unix\", nil, dst)")
	g.gen.P("}")
}
