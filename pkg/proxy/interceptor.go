/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proxy

import "github.com/golang/protobuf/protoc-gen-go/generator"

// generateProxyInterceptor is a method of the proxy struct that satisfies the
// grpc.UnaryInterceptor interface. This allows us to make use of the tls
// information from the provider to include it with each subsequent request
// from the proxy. This is also where we handle some of the routing decisions,
// namely being able to filter on the supported service and handling the
// 'proxyfrom' metadata field to prevent infinite loops.
func (g *proxy) generateProxyInterceptor(serviceName string) {
	tName := generator.CamelCase(serviceName + "_proxy")
	g.gen.P("func (p *" + tName + ") UnaryInterceptor() grpc.UnaryServerInterceptor {")
	g.gen.P("return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {")
	g.gen.P("md, _ := metadata.FromIncomingContext(ctx)")
	g.gen.P("if _, ok := md[\"proxyfrom\"]; ok {")
	g.gen.P("log.Printf(\"handling request: %s\", info.FullMethod)")
	g.gen.P("return handler(ctx, req)")
	g.gen.P("}")
	g.gen.P("log.Printf(\"proxy request: %s\", info.FullMethod)")
	g.gen.P("ca, err := p.Provider.GetCA()")
	g.gen.P("if err != nil {")
	g.gen.P("	return nil, err")
	g.gen.P("}")
	g.gen.P("certs, err := p.Provider.GetCertificate(nil)")
	g.gen.P("if err != nil {")
	g.gen.P("  return nil, err")
	g.gen.P("}")
	g.gen.P("tlsConfig, err := tls.New(")
	g.gen.P("  tls.WithClientAuthType(tls.Mutual),")
	g.gen.P("  tls.WithCACertPEM(ca),")
	g.gen.P("  tls.WithKeypair(*certs),")
	g.gen.P(")")
	g.gen.P("return p.Proxy(ctx, info.FullMethod, credentials.NewTLS(tlsConfig), req)")
	g.gen.P("}")
	g.gen.P("}")
	g.gen.P("")
}
