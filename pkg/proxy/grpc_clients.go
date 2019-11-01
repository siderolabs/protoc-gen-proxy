/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proxy

import (
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// generateLocalClient generates a local ( as in served by the host itself )
// client to connect with the other node local grpc endpoints.
func (g *proxy) generateLocalClient(serviceName, pkgName string) {
	// type
	g.P(g.GrpcClient, "type Local"+serviceName+"Client struct {")
	g.P(g.GrpcClient, pkgName+"."+serviceName+"Client")
	g.P(g.GrpcClient, "}")
	g.P(g.GrpcClient, "")

	// constructor
	g.P(g.GrpcClient, "func NewLocal"+serviceName+"Client() ("+pkgName+"."+serviceName+"Client, error) {")
	g.P(g.GrpcClient, "conn, err := grpc.Dial(\"unix:\"+constants."+serviceName+"SocketPath,")
	g.P(g.GrpcClient, "grpc.WithInsecure(),")
	g.P(g.GrpcClient, "grpc.WithContextDialer(noProxyDialer),")
	g.P(g.GrpcClient, ")")
	g.P(g.GrpcClient, "if err != nil {")
	g.P(g.GrpcClient, "return nil, err")
	g.P(g.GrpcClient, "}")
	g.P(g.GrpcClient, "return &Local"+serviceName+"Client{")
	g.P(g.GrpcClient, serviceName+"Client: "+pkgName+".New"+serviceName+"Client(conn),")
	g.P(g.GrpcClient, "}, nil")
	g.P(g.GrpcClient, "}")
	g.P(g.GrpcClient, "")
}

// generateClientMethods generates the methods to satisfy the XXClient interface.
// These methods are part of the Local<serviceName>Client struct.
func (g *proxy) generateClientMethods(serviceName, pkgName string, method *descriptor.MethodDescriptorProto) {
	// method arguments
	var args strings.Builder
	args.WriteString("(")
	args.WriteString("ctx context.Context")
	args.WriteString(", in *" + g.typeName(method.GetInputType()))
	args.WriteString(", opts ...grpc.CallOption")
	args.WriteString(")")

	// method returns
	var returns strings.Builder
	returns.WriteString("(")
	if method.GetServerStreaming() || method.GetClientStreaming() {
		returns.WriteString(pkgName + "." + serviceName + "_" + generator.CamelCase(method.GetName()) + "Client")
	} else {
		returns.WriteString("*" + g.typeName(method.GetOutputType()))
	}
	returns.WriteString(", error")
	returns.WriteString(")")

	// method definition
	g.P(g.GrpcClient, "func (c *Local"+serviceName+"Client) "+method.GetName()+args.String()+returns.String()+"{")
	g.P(g.GrpcClient, "return c."+serviceName+"Client."+method.GetName()+"(ctx, in, opts...)")
	g.P(g.GrpcClient, "}")

}
