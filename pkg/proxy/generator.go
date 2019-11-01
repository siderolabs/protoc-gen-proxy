/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2010 The Go Authors.  All rights reserved.
// https://github.com/golang/protobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package proxy

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

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
	ProxySwitch         *bytes.Buffer
	ProxyFns            *bytes.Buffer
	InitClients         *bytes.Buffer
	Clients             *bytes.Buffer
	Registrator         *bytes.Buffer
	RegistratorRegister *bytes.Buffer

	GrpcClient *bytes.Buffer
	GrpcServer *bytes.Buffer

	WrapperFns *bytes.Buffer

	gen *generator.Generator
}

// Name returns the name of this plugin, "proxy".
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
	g.ProxySwitch = new(bytes.Buffer)
	g.ProxyFns = new(bytes.Buffer)
	g.InitClients = new(bytes.Buffer)
	g.Clients = new(bytes.Buffer)
	g.WrapperFns = new(bytes.Buffer)
	g.Registrator = new(bytes.Buffer)
	g.RegistratorRegister = new(bytes.Buffer)
	g.GrpcClient = new(bytes.Buffer)
	g.GrpcServer = new(bytes.Buffer)
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

// P writes to internal (*proxy) buffers that later get consumed by
// g.gen.P(buffer.String())
func (g *proxy) P(w *bytes.Buffer, str ...interface{}) {
	for _, v := range str {
		g.printAtom(w, v)
	}
	w.WriteByte('\n')
}

// printAtom prints the (atomic, non-annotation) argument to the generated output.
func (g *proxy) printAtom(w *bytes.Buffer, v interface{}) {
	switch v := v.(type) {
	case string:
		w.WriteString(v)
	case *string:
		w.WriteString(*v)
	case bool:
		fmt.Fprint(w, v)
	case *bool:
		fmt.Fprint(w, *v)
	case int:
		fmt.Fprint(w, v)
	case *int32:
		fmt.Fprint(w, *v)
	case *int64:
		fmt.Fprint(w, *v)
	case float64:
		fmt.Fprint(w, v)
	case *float64:
		fmt.Fprint(w, *v)
	case generator.GoPackageName:
		w.WriteString(string(v))
	case generator.GoImportPath:
		w.WriteString(strconv.Quote(string(v)))
	default:
		g.gen.Fail(fmt.Sprintf("unknown type in printer: %T", v))
	}
}

// GenerateImports generates the import declaration for this file.
func (g *proxy) GenerateImports(file *generator.FileDescriptor) {
}

// Generate is the main entrypoint to the plugin. This is where all
// the magic happens.
func (g *proxy) Generate(file *generator.FileDescriptor) {
	// Try to filter out non-builtins
	// ex, we don't want to do this for  google/protobuf/empty.proto
	if strings.Contains(*file.Package, "google") {
		return
	}

	// If we're dealing with the actual file to generate,
	// we'll print out everything we've generated so far ( all stored
	// bytes.Buffers ) and we'll generate the high level wrappers
	// like `Proxy()`, `UnaryInterceptor()`, `Runner()`
	for _, f := range g.gen.Request.FileToGenerate {
		if file.GetName() == f {
			g.generate(file)
			return
		}
	}

	// Otherwise, we'll generate all the fun per package/proto
	// imports and switch statements so we can satisfy the
	// - switch statement cases
	// - various function definitions
	// - client creation functions
	for _, service := range file.FileDescriptorProto.Service {
		serviceName := generator.CamelCase(service.GetName())

		// g.ProxySwitch
		g.generateSwitchStatement(serviceName, file.GetPackage(), service.Method)

		// g.ProxyFns
		g.generateServiceFuncType(serviceName)
		g.generateServiceRunner(serviceName)
		g.generateProxyClientStruct(serviceName, file.GetPackage())

		for _, method := range service.Method {
			// No support for streaming stuff yet
			if method.GetServerStreaming() || method.GetClientStreaming() {
				continue
			}

			// g.ProxyFns
			g.generateServiceFunc(serviceName, method)
		}

		// g.Clients
		g.generateClientFns(serviceName, file.GetPackage())

		// g.Registrator
		g.generateRegistrator(service, file.GetPackage())

		// g.RegistratorRegister
		g.generateRegistratorRegister(service, file.GetPackage())

		for _, method := range service.Method {
			// Need to generate a full set of methods to satisfy
			// the server interface

			// g.GrpcServer
			g.generateServerMethods(serviceName, file.GetPackage(), method)
		}

		// g.GrpcClient
		g.generateLocalClient(serviceName, file.GetPackage())

		for _, method := range service.Method {
			// Need to generate a full set of methods to satisfy
			// the client interface

			// g.GrpcClient
			g.generateClientMethods(serviceName, file.GetPackage(), method)
		}
	}
}

func (g *proxy) generate(file *generator.FileDescriptor) {
	g.gen.AddImport("io")
	g.gen.AddImport("sync")
	g.gen.AddImport("google.golang.org/grpc/metadata")
	g.gen.AddImport("google.golang.org/grpc/credentials")
	g.gen.AddImport("github.com/hashicorp/go-multierror")
	// Support `provider`
	g.gen.AddImport("github.com/talos-systems/talos/pkg/grpc/tls")
	// Support for socket paths
	g.gen.AddImport("github.com/talos-systems/talos/pkg/constants")

	g.generateProxyStruct(file.GetPackage())

	g.generateProxyInterceptor(file.GetPackage())

	g.generateProxyRouter(file.GetPackage())

	g.generateStreamCopyHelper()

	g.generateNoProxyDialer()

	g.gen.P(g.ProxyFns.String())
	g.gen.P("")

	g.gen.P(g.Clients.String())
	g.gen.P("")

	g.gen.P("type Registrator struct {")
	g.gen.P(g.Registrator.String())
	g.gen.P("}")
	g.gen.P("")

	g.gen.P("func (r *Registrator) Register(s *grpc.Server) {")
	g.gen.P(g.RegistratorRegister.String())
	g.gen.P("}")
	g.gen.P("")

	g.gen.P(g.GrpcServer.String())
	g.gen.P("")

	g.gen.P(g.GrpcClient.String())
}
