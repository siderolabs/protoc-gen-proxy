/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proxy

import (
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// generateRegistrator generates the embedded types for the registrator.
func (g *proxy) generateRegistrator(service *descriptor.ServiceDescriptorProto, pkgName string) {
	serviceName := generator.CamelCase(service.GetName())
	g.P(g.Registrator, pkgName+"."+serviceName+"Client")
}

// generateRegistratorRegister generates the grpc server registration calls.
func (g *proxy) generateRegistratorRegister(service *descriptor.ServiceDescriptorProto, pkgName string) {
	serviceName := generator.CamelCase(service.GetName())
	g.P(g.RegistratorRegister, pkgName+"."+"Register"+serviceName+"Server(s,r)")
}

// generateGRPCServers generates the methods to satisfy the XXServer interface.
// This differs ever so slightly from the XXClient interface.
func (g *proxy) generateServerMethods(serviceName, pkgName string, method *descriptor.MethodDescriptorProto) {
	if method.GetServerStreaming() || method.GetClientStreaming() {
		g.generateServerStreamMethods(serviceName, pkgName, method)
	} else {
		g.generateServerUnaryMethods(serviceName, pkgName, method)
	}
}

func (g *proxy) generateServerUnaryMethods(serviceName, pkgName string, method *descriptor.MethodDescriptorProto) {
	var serverArgs strings.Builder
	serverArgs.WriteString("(")
	serverArgs.WriteString("ctx context.Context, ")
	serverArgs.WriteString("in *" + g.typeName(method.GetInputType()))
	serverArgs.WriteString(")")

	var serverReturns strings.Builder
	serverReturns.WriteString("(")
	serverReturns.WriteString("*" + g.typeName(method.GetOutputType()) + ", ")
	serverReturns.WriteString("error")
	serverReturns.WriteString(")")

	g.P(g.GrpcServer, "func (r *Registrator) "+method.GetName()+serverArgs.String()+serverReturns.String()+"{")
	g.P(g.GrpcServer, "return r."+serviceName+"Client."+method.GetName()+"(ctx, in)")
	g.P(g.GrpcServer, "}")
}
func (g *proxy) generateServerStreamMethods(serviceName, pkgName string, method *descriptor.MethodDescriptorProto) {
	var serverArgs strings.Builder
	serverArgs.WriteString("(")
	serverArgs.WriteString("in *" + g.typeName(method.GetInputType()))
	serverArgs.WriteString(", srv " + pkgName + "." + serviceName + "_" + generator.CamelCase(method.GetName()) + "Server, ")
	serverArgs.WriteString(")")

	var serverReturns strings.Builder
	serverReturns.WriteString("(")
	serverReturns.WriteString("error")
	serverReturns.WriteString(")")

	g.P(g.GrpcServer, "func (r *Registrator) "+method.GetName()+
		serverArgs.String()+serverReturns.String()+"{")

	g.P(g.GrpcServer, "client, err := r."+serviceName+"Client."+method.GetName()+"(srv.Context(), in)")
	g.P(g.GrpcServer, "if err != nil {")
	g.P(g.GrpcServer, "return err")
	g.P(g.GrpcServer, "}")
	g.P(g.GrpcServer, "var msg "+g.typeName(method.GetOutputType()))
	g.P(g.GrpcServer, "return copyClientServer(&msg, client, srv)")

	g.P(g.GrpcServer, "}")
}
