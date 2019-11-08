/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proxy

import (
	"strings"

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
