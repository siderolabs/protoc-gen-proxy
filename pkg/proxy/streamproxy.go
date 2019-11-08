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

// generateStreamProxyRouter creates the routing part of the proxy. That is it
// enables us to map the incoming grpc method to the function/client call
// so we can properly call the proper rpc endpoint.
func (g *proxy) generateStreamProxyRouter(serviceName string) {

	var args strings.Builder
	args.WriteString("(")
	args.WriteString("ss " + grpcPkg + ".ServerStream, ")
	args.WriteString("method string, ")
	args.WriteString("creds credentials.TransportCredentials, ")
	args.WriteString("srv interface{}, ")
	args.WriteString("opts ..." + grpcPkg + ".CallOption")
	args.WriteString(")")

	var returns strings.Builder
	returns.WriteString("error")

	g.gen.P("func (p *" + generator.CamelCase(serviceName+"_proxy") + ") StreamProxy " + args.String() + returns.String() + "{")
	g.gen.P("var (")
	g.gen.P("err error")
	g.gen.P("errors *go_multierror.Error")
	g.gen.P("ok bool")
	g.gen.P("targets []string")
	g.gen.P(")")
	g.gen.P("")

	// Parse targets from incoming metadata/context
	g.gen.P("md, _ := metadata.FromIncomingContext(ss.Context())")
	g.gen.P("// default to target node specified in config or on cli")
	g.gen.P("if targets, ok = md[\"targets\"]; !ok {")
	g.gen.P("targets = md[\":authority\"]")
	g.gen.P("}")
	g.gen.P("// Can discuss more on how to handle merging multiple streams later")
	g.gen.P("// but for now, ensure we only deal with a single target")
	g.gen.P("if len(targets) > 1 {")
	g.gen.P("targets = targets[:1]")
	g.gen.P("}")
	g.gen.P("")

	// Set up client connections
	g.gen.P("proxyMd := metadata.New(make(map[string]string))")
	g.gen.P("proxyMd.Set(\"proxyfrom\", md[\":authority\"]...)")
	g.gen.P("")

	// Handle routes
	g.gen.P("switch method {")
	g.gen.P(g.StreamProxySwitch.String())
	g.gen.P("}")
	g.gen.P("")
	g.gen.P("if err != nil {")
	g.gen.P("errors = go_multierror.Append(errors, err)")
	g.gen.P("}")
	g.gen.P("return errors.ErrorOrNil()")
	g.gen.P("}")
}

func (g *proxy) generateStreamSwitchStatement(serviceName, pkgName string, methods []*pb.MethodDescriptorProto) {
	for _, method := range methods {
		// Only handle streaming methods
		switch {
		case method.GetServerStreaming():
		case method.GetClientStreaming():
		default:
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
		g.P(g.StreamProxySwitch, "case \""+sname+"\":")
		g.P(g.StreamProxySwitch, "// Initialize target clients")
		g.P(g.StreamProxySwitch, "clients, err := create"+serviceName+"Client(targets, creds, proxyMd)")
		g.P(g.StreamProxySwitch, "if err != nil {")
		g.P(g.StreamProxySwitch, "break")
		g.P(g.StreamProxySwitch, "}")

		g.P(g.StreamProxySwitch, "m := new("+g.typeName(method.GetInputType())+")")
		g.P(g.StreamProxySwitch, "if err := ss.RecvMsg(m); err != nil {")
		g.P(g.StreamProxySwitch, "return err")
		g.P(g.StreamProxySwitch, "}")

		g.P(g.StreamProxySwitch, "// artificially limit this to only the first client/target until")
		g.P(g.StreamProxySwitch, "// we get multi-stream stuff sorted")
		//	g.P(g.StreamProxySwitch, "outgoingContext := metadata.NewOutgoingContext(ss.Context(), proxyMd)")
		g.P(g.StreamProxySwitch, "clientStream, err := clients[0].Conn."+method.GetName()+"(clients[0].Context, m)")
		g.P(g.StreamProxySwitch, "if err != nil {")
		g.P(g.StreamProxySwitch, "return err")
		g.P(g.StreamProxySwitch, "}")
		g.P(g.StreamProxySwitch, "var msg "+g.typeName(method.GetOutputType()))
		g.P(g.StreamProxySwitch, "return copyClientServer(&msg, clientStream, ss.(grpc.ServerStream))")

		/*
			var fnArgs strings.Builder
			fnArgs.WriteString("(")
			fnArgs.WriteString("clients, ")
			fnArgs.WriteString("in, ")
			fnArgs.WriteString("proxy" + generator.CamelCase(method.GetName()))
			fnArgs.WriteString(")")

			g.P(g.StreamProxySwitch, "resp := &"+g.typeName(method.GetOutputType())+"{}")
			g.P(g.StreamProxySwitch, "msgs, err = proxy"+serviceName+"Runner"+fnArgs.String())
			g.P(g.StreamProxySwitch, "for _, msg := range msgs {")
			g.P(g.StreamProxySwitch, "resp.Response = append(resp.Response, msg.(*"+g.typeName(method.GetOutputType())+").Response[0])")
			g.P(g.StreamProxySwitch, "}")
			g.P(g.StreamProxySwitch, "response = resp")
		*/
	}
}
