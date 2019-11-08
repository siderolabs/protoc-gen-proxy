/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proxy

import "github.com/golang/protobuf/protoc-gen-go/generator"

// generateUnaryInterceptor is a method of the proxy struct that satisfies the
// grpc.UnaryInterceptor interface. This allows us to make use of the tls
// information from the provider to include it with each subsequent request
// from the proxy. This is also where we handle some of the routing decisions,
// namely being able to filter on the supported service and handling the
// 'proxyfrom' metadata field to prevent infinite loops.
func (g *proxy) generateUnaryInterceptor(serviceName string) {
	tName := generator.CamelCase(serviceName + "_proxy")
	g.gen.P("func (p *" + tName + ") UnaryInterceptor() grpc.UnaryServerInterceptor {")
	g.gen.P("return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {")
	g.gen.P("md, _ := metadata.FromIncomingContext(ctx)")
	g.gen.P("if _, ok := md[\"proxyfrom\"]; ok {")
	g.gen.P("return handler(ctx, req)")
	g.gen.P("}")
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
	g.gen.P("return p.UnaryProxy(ctx, info.FullMethod, credentials.NewTLS(tlsConfig), req)")
	g.gen.P("}")
	g.gen.P("}")
	g.gen.P("")
}

// generateStreamInterceptor is a method of the proxy struct that satisfies the
// grpc.UnaryInterceptor interface. This allows us to make use of the tls
// information from the provider to include it with each subsequent request
// from the proxy. This is also where we handle some of the routing decisions,
// namely being able to filter on the supported service and handling the
// 'proxyfrom' metadata field to prevent infinite loops.
func (g *proxy) generateStreamInterceptor(serviceName string) {
	tName := generator.CamelCase(serviceName + "_proxy")
	g.gen.P("func (p *" + tName + ") StreamInterceptor() grpc.StreamServerInterceptor {")
	g.gen.P("return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {")
	g.gen.P("md, _ := metadata.FromIncomingContext(ss.Context())")
	g.gen.P("if _, ok := md[\"proxyfrom\"]; ok {")
	g.gen.P("return handler(srv, ss)")
	g.gen.P("}")
	g.gen.P("ca, err := p.Provider.GetCA()")
	g.gen.P("if err != nil {")
	g.gen.P("	return err")
	g.gen.P("}")
	g.gen.P("certs, err := p.Provider.GetCertificate(nil)")
	g.gen.P("if err != nil {")
	g.gen.P("  return err")
	g.gen.P("}")
	g.gen.P("tlsConfig, err := tls.New(")
	g.gen.P("  tls.WithClientAuthType(tls.Mutual),")
	g.gen.P("  tls.WithCACertPEM(ca),")
	g.gen.P("  tls.WithKeypair(*certs),")
	g.gen.P(")")
	g.gen.P("return p.StreamProxy(ss, info.FullMethod, credentials.NewTLS(tlsConfig), srv)")
	g.gen.P("}")
	g.gen.P("}")
	g.gen.P("")
}
