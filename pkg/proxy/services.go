/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proxy

import (
	"strings"

	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

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
	g.P(g.ProxyFns, "type runner"+generator.CamelCase(serviceName+"_fn")+" func"+args.String())
	g.P(g.ProxyFns, "")
}

// generateServiceFunc is a function generated for each service defined in the
// proto file. The function signature satisfies the runnerfn type.
func (g *proxy) generateServiceFunc(serviceName string, method *pb.MethodDescriptorProto) {
	// skip support for deprecated methods
	if method.GetOptions().GetDeprecated() {
		return
	}
	var args strings.Builder
	args.WriteString("(")
	args.WriteString("client *proxy" + serviceName + "Client, ")
	args.WriteString("in interface{}, ")
	args.WriteString("wg *sync.WaitGroup, ")
	args.WriteString("respCh chan proto.Message, ")
	args.WriteString("errCh chan error")
	args.WriteString(")")

	g.P(g.ProxyFns, "func proxy"+generator.CamelCase(method.GetName())+args.String()+"{")
	g.P(g.ProxyFns, "defer wg.Done()")
	g.P(g.ProxyFns, "resp, err := client.Conn."+method.GetName()+"(client.Context, in.(*"+g.typeName(method.GetInputType())+"))")
	g.P(g.ProxyFns, "if err != nil {")
	g.P(g.ProxyFns, "errCh<-err")
	g.P(g.ProxyFns, "return")
	g.P(g.ProxyFns, "}")
	// TODO: See if we can better abstract this
	g.P(g.ProxyFns, "resp.Response[0].Metadata = &NodeMetadata{Hostname: client.Target}")
	g.P(g.ProxyFns, "respCh<-resp")
	g.P(g.ProxyFns, "}")
	g.P(g.ProxyFns, "")
}

// generateServiceRunner is the function that handles the client calls and response
// aggregation.
func (g *proxy) generateServiceRunner(serviceName string) {
	var args strings.Builder
	args.WriteString("(")
	args.WriteString("clients []*proxy" + serviceName + "Client, ")
	args.WriteString("in interface{}, ")
	args.WriteString("runner runner" + generator.CamelCase(serviceName+"_fn"))
	args.WriteString(")")

	var returns strings.Builder
	returns.WriteString("(")
	returns.WriteString("[]proto.Message, ")
	returns.WriteString("error")
	returns.WriteString(")")

	g.P(g.ProxyFns, "func proxy"+generator.CamelCase(serviceName+"_runner")+args.String()+returns.String()+"{")
	g.P(g.ProxyFns, "var (")
	g.P(g.ProxyFns, "errors *go_multierror.Error")
	g.P(g.ProxyFns, "wg sync.WaitGroup")
	g.P(g.ProxyFns, ")")

	g.P(g.ProxyFns, "respCh := make(chan proto.Message, len(clients))")
	g.P(g.ProxyFns, "errCh := make(chan error, len(clients))")
	g.P(g.ProxyFns, "wg.Add(len(clients))")

	g.P(g.ProxyFns, "for _, client := range clients {")
	g.P(g.ProxyFns, "go runner(client, in, &wg, respCh, errCh)")
	g.P(g.ProxyFns, "}")

	g.P(g.ProxyFns, "wg.Wait()")
	g.P(g.ProxyFns, "close(respCh)")
	g.P(g.ProxyFns, "close(errCh)")
	g.P(g.ProxyFns, "")

	g.P(g.ProxyFns, "var response []proto.Message")
	g.P(g.ProxyFns, "for resp := range respCh {")
	g.P(g.ProxyFns, "response = append(response, resp)")
	g.P(g.ProxyFns, "}")

	g.P(g.ProxyFns, "for err := range errCh {")
	g.P(g.ProxyFns, "errors = go_multierror.Append(errors, err)")
	g.P(g.ProxyFns, "}")

	g.P(g.ProxyFns, "return response, errors.ErrorOrNil()")
	g.P(g.ProxyFns, "}")
	g.P(g.ProxyFns, "")
}

// generateClientFns generates the helper functions to instantiate a slice of service oriented client connections.
func (g *proxy) generateClientFns(serviceName, pkgName string) {
	fullServName := serviceName
	if pkgName != "" {
		fullServName = pkgName + "." + fullServName
	}
	g.P(g.Clients, "")
	g.P(g.Clients, "func create"+serviceName+"Client(targets []string, creds credentials.TransportCredentials, proxyMd metadata.MD) ([]*proxy"+serviceName+"Client ,error){")
	g.P(g.Clients, "var errors *go_multierror.Error")
	g.P(g.Clients, "clients := make([]*proxy"+serviceName+"Client, 0, len(targets))")
	g.P(g.Clients, "for _, target := range targets {")
	g.P(g.Clients, "c := &proxy"+serviceName+"Client{")
	g.P(g.Clients, "// TODO change the context to be more useful ( ex cancelable )")
	g.P(g.Clients, "Context: metadata.NewOutgoingContext(context.Background(), proxyMd),")
	g.P(g.Clients, "Target:  target,")
	g.P(g.Clients, "}")
	g.P(g.Clients, "// TODO: i think we potentially leak a client here,")
	g.P(g.Clients, "// we should close the request // cancel the context if it errors")
	g.P(g.Clients, "// Explicitly set OSD port")
	g.P(g.Clients, "conn, err := grpc.Dial(fmt.Sprintf(\"%s:%d\", target, 50000), grpc.WithTransportCredentials(creds))")
	g.P(g.Clients, "if err != nil {")
	g.P(g.Clients, "// TODO: probably worth wrapping err to add some context about the target")
	g.P(g.Clients, "errors = go_multierror.Append(errors, err)")
	g.P(g.Clients, "continue")
	g.P(g.Clients, "}")
	g.P(g.Clients, "c.Conn = "+pkgName+".New"+serviceName+"Client(conn)")
	g.P(g.Clients, "clients = append(clients, c)")
	g.P(g.Clients, "}")
	g.P(g.Clients, "return clients, errors.ErrorOrNil()")
	g.P(g.Clients, "}")
}
