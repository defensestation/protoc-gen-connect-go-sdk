// Copyright 2021-2024 The Connect Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// protoc-gen-connect-go is a plugin for the Protobuf compiler that generates
// Go code. To use it, build this program and make it available on your PATH as
// protoc-gen-connect-go.
//
// The 'connect-go' suffix becomes part of the arguments for the Protobuf
// compiler. To generate the base Go types and Connect code using protoc:
//
//	protoc --go_out=gen --connect-go_out=gen path/to/file.proto
//
// With [buf], your buf.gen.yaml will look like this:
//
//	version: v1
//	plugins:
//	  - name: go
//	    out: gen
//	  - name: connect-go
//	    out: gen
//
// This generates service definitions for the Protobuf types and services
// defined by file.proto. If file.proto defines the foov1 Protobuf package, the
// invocations above will write output to:
//
//	gen/path/to/file.pb.go
//	gen/path/to/connectfoov1/file.connect.go
//
// [buf]: https://buf.build

// changes made
// generatedFilenameExtension = ".connect.go" --> ".client.go"
// generatedPackageSuffix     = "connect" --> ""
// all modified code is comment like :::Modified:::
package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	connect "connectrpc.com/connect"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	// skd paths
	sdkConfigPackage = protogen.GoImportPath("github.com/defensestation/internal-sdk-go/config")
	sdkdsInterceptorPackage = protogen.GoImportPath("github.com/defensestation/ds-sdk-go/internal/middleware/interceptors")
	sdkendpointPackage = protogen.GoImportPath("github.com/defensestation/ds-sdk-go/internal/endpoints")

	contextPackage = protogen.GoImportPath("context")
	errorsPackage  = protogen.GoImportPath("errors")
	httpPackage    = protogen.GoImportPath("net/http")
	stringsPackage = protogen.GoImportPath("strings")
	connectPackage = protogen.GoImportPath("connectrpc.com/connect")
	protoreflectPkg = protogen.GoImportPath("google.golang.org/protobuf/reflect/protoreflect")

	generatedFilenameExtension = ".client.go"  //:::Modified:::
	generatedPackageSuffix     = ""   //:::Modified:::

	usage = "See https://connectrpc.com/docs/go/getting-started to learn how to use this plugin.\n\nFlags:\n  -h, --help\tPrint this help and exit.\n      --version\tPrint the version and exit."

	commentWidth = 97 // leave room for "// "

	// To propagate top-level comments, we need the field number of the syntax
	// declaration and the package name in the file descriptor.
	protoSyntaxFieldNum  = 12
	protoPackageFieldNum = 2
)


// method to generate sdk client

func generateSDKClientImplementation(g *protogen.GeneratedFile, service *protogen.Service, names names) {
	constName := fmt.Sprintf("%sName", service.Desc.Name())
	g.P("// sdk user client with interceptors")
	g.P("func NewClient(ctx "+ g.QualifiedGoIdent(contextPackage.Ident("Context")) +", cfg *"+ g.QualifiedGoIdent(sdkConfigPackage.Ident("Config")) +") ("+ names.Client +", error) {")
	g.P("interceptors, err := "+ g.QualifiedGoIdent(sdkdsInterceptorPackage.Ident("LoadInterceptors")) +"(ctx, cfg, "+ constName +")") 
	g.P("if err != nil {")
	g.P("return nil, err")
	g.P("}")
	g.P("return " + names.ClientConstructor + "(")
	g.P(g.QualifiedGoIdent(httpPackage.Ident("DefaultClient")) + ",")
	g.P(g.QualifiedGoIdent(sdkendpointPackage.Ident("Resolve")) +"(cfg, " + constName + "),")
	g.P(g.QualifiedGoIdent(connectPackage.Ident("WithGRPC")) + "(),")
	g.P("interceptors,")
	g.P("), nil")
	g.P("}")
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Fprintln(os.Stdout, connect.Version)
		os.Exit(0)
	}
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		fmt.Fprintln(os.Stdout, usage)
		os.Exit(0)
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	protogen.Options{}.Run(
		func(plugin *protogen.Plugin) error {
			plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL) | uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
			plugin.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_PROTO2
			plugin.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2023
			for _, file := range plugin.Files {
				if file.Generate {
					generate(plugin, file)
				}
			}
			return nil
		},
	)
}

func generate(plugin *protogen.Plugin, file *protogen.File) {
	if len(file.Services) == 0 {
		return
	}
	file.GoPackageName += generatedPackageSuffix

	generatedFilenamePrefixToSlash := filepath.ToSlash(file.GeneratedFilenamePrefix)
	file.GeneratedFilenamePrefix = path.Join(
		path.Dir(generatedFilenamePrefixToSlash),
		// string(file.GoPackageName), // ensure to create in same dir   //:::Modified:::
		path.Base(generatedFilenamePrefixToSlash),
	)
	generatedFile := plugin.NewGeneratedFile(
		file.GeneratedFilenamePrefix+generatedFilenameExtension,
		protogen.GoImportPath(path.Join(
			string(file.GoImportPath), 
			// string(file.GoPackageName), // ensure to create in same dir   //:::Modified:::
		)),
	)
	generatedFile.Import(file.GoImportPath)
	generatePreamble(generatedFile, file)
	generateServiceNameConstants(generatedFile, file.Services)
	generateServiceNameVariables(generatedFile, file)
	for _, service := range file.Services {
		generateService(generatedFile, service)
	}
}

func generatePreamble(g *protogen.GeneratedFile, file *protogen.File) {
	syntaxPath := protoreflect.SourcePath{protoSyntaxFieldNum}
	syntaxLocation := file.Desc.SourceLocations().ByPath(syntaxPath)
	for _, comment := range syntaxLocation.LeadingDetachedComments {
		leadingComments(g, protogen.Comments(comment), false /* deprecated */)
	}
	g.P()
	leadingComments(g, protogen.Comments(syntaxLocation.LeadingComments), false /* deprecated */)
	g.P()

	programName := filepath.Base(os.Args[0])
	// Remove .exe suffix on Windows so that generated code is stable, regardless
	// of whether it was generated on a Windows machine or not.
	if ext := filepath.Ext(programName); strings.ToLower(ext) == ".exe" {
		programName = strings.TrimSuffix(programName, ext)
	}
	g.P("// Code generated by ", programName, ". DO NOT EDIT.")
	g.P("//")
	if file.Proto.GetOptions().GetDeprecated() {
		wrapComments(g, file.Desc.Path(), " is a deprecated file.")
	} else {
		g.P("// Source: ", file.Desc.Path())
	}
	g.P()

	pkgPath := protoreflect.SourcePath{protoPackageFieldNum}
	pkgLocation := file.Desc.SourceLocations().ByPath(pkgPath)
	for _, comment := range pkgLocation.LeadingDetachedComments {
		leadingComments(g, protogen.Comments(comment), false /* deprecated */)
	}
	g.P()
	leadingComments(g, protogen.Comments(pkgLocation.LeadingComments), false /* deprecated */)

	g.P("package ", file.GoPackageName)
	g.P()
	wrapComments(g, "This is a compile-time assertion to ensure that this generated file ",
		"and the connect package are compatible. If you get a compiler error that this constant ",
		"is not defined, this code was generated with a version of connect newer than the one ",
		"compiled into your binary. You can fix the problem by either regenerating this code ",
		"with an older version of connect or updating the connect version compiled into your binary.")
	g.P("const _ = ", connectPackage.Ident("IsAtLeastVersion1_13_0"))
	g.P()
}

func generateServiceNameConstants(g *protogen.GeneratedFile, services []*protogen.Service) {
	var numMethods int
	g.P("const (")
	for _, service := range services {
		constName := fmt.Sprintf("%sName", service.Desc.Name())
		wrapComments(g, constName, " is the fully-qualified name of the ",
			service.Desc.Name(), " service.")
		g.P(constName, ` = "`, service.Desc.FullName(), `"`)
		numMethods += len(service.Methods)
	}
	g.P(")")
	g.P()

	if numMethods == 0 {
		return
	}
	wrapComments(g, "These constants are the fully-qualified names of the RPCs defined in this package. ",
		"They're exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.")
	g.P("//")
	wrapComments(g, "Note that these are different from the fully-qualified method names used by ",
		"google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to ",
		"reflection-formatted method names, remove the leading slash and convert the ",
		"remaining slash to a period.")
	g.P("const (")
	for _, service := range services {
		for _, method := range service.Methods {
			// The runtime exposes this value as Spec.Procedure, so we should use the
			// same term here.
			wrapComments(g, procedureConstName(method), " is the fully-qualified name of the ",
				service.Desc.Name(), "'s ", method.Desc.Name(), " RPC.")
			g.P(procedureConstName(method), ` = "`, fmt.Sprintf("/%s/%s", service.Desc.FullName(), method.Desc.Name()), `"`)
		}
	}
	g.P(")")
	g.P()
}

func generateServiceNameVariables(g *protogen.GeneratedFile, file *protogen.File) {
	wrapComments(g, "These variables are the protoreflect.Descriptor objects for the RPCs defined in this package.")
	g.P("var (")
	for _, service := range file.Services {
		serviceDescName := unexport(fmt.Sprintf("%sServiceDescriptor", service.Desc.Name()))
		g.P(serviceDescName, ` `, protoreflectPkg.Ident("ServiceDescriptor"))
		for _, method := range service.Methods {
			g.P(procedureVarMethodDescriptor(method), ` `, protoreflectPkg.Ident("MethodDescriptor"))
		}
	}
	g.P(")")
	g.P("func loadVars() {")
	for _, service := range file.Services {
		serviceDescName := unexport(fmt.Sprintf("%sServiceDescriptor", service.Desc.Name()))
		g.P(serviceDescName, ` = `,
			g.QualifiedGoIdent(file.GoDescriptorIdent),
			`.Services().ByName("`, service.Desc.Name(), `")`)
		for _, method := range service.Methods {
			g.P(procedureVarMethodDescriptor(method), ` = `,
				serviceDescName,
				`.Methods().ByName("`, method.Desc.Name(), `")`)
		}
	}
	g.P("}")

	// :::Modified:::
	g.P("init() {")
	for _, _ = range file.Services {
		g.P(g.QualifiedGoIdent(file.GoDescriptorIdent),`_init()`)
	}
	g.P("loadVars()")
	g.P("}")

}

func generateService(g *protogen.GeneratedFile, service *protogen.Service) {
	names := newNames(service)
	// generateSDKClientImplementation(g, service, names)
	generateClientInterface(g, service, names)
	generateClientImplementation(g, service, names)
	// generateServerInterface(g, service, names)   //:::Modified:::
	// generateServerConstructor(g, service, names)   //:::Modified:::
	// generateUnimplementedServerImplementation(g, service, names)   //:::Modified:::
}

func generateClientInterface(g *protogen.GeneratedFile, service *protogen.Service, names names) {
	wrapComments(g, names.Client, " is a client for the ", service.Desc.FullName(), " service.")
	if isDeprecatedService(service) {
		g.P("//")
		deprecated(g)
	}
	g.AnnotateSymbol(names.Client, protogen.Annotation{Location: service.Location})
	g.P("type ", names.Client, " interface {")
	for _, method := range service.Methods {
		g.AnnotateSymbol(names.Client+"."+method.GoName, protogen.Annotation{Location: method.Location})
		leadingComments(
			g,
			method.Comments.Leading,
			isDeprecatedMethod(method),
		)
		g.P(clientSignature(g, method, false /* named */))
	}
	g.P("}")
	g.P()
}

func generateClientImplementation(g *protogen.GeneratedFile, service *protogen.Service, names names) {
	clientOption := connectPackage.Ident("ClientOption")

	// Client constructor.
	wrapComments(g, names.ClientConstructor, " constructs a client for the ", service.Desc.FullName(),
		" service. By default, it uses the Connect protocol with the binary Protobuf Codec, ",
		"asks for gzipped responses, and sends uncompressed requests. ",
		"To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or ",
		"connect.WithGRPCWeb() options.")
	g.P("//")
	wrapComments(g, "The URL supplied here should be the base URL for the Connect or gRPC server ",
		"(for example, http://api.acme.com or https://acme.com/grpc).")
	if isDeprecatedService(service) {
		g.P("//")
		deprecated(g)
	}
	g.P("func ", names.ClientConstructor, " (httpClient ", connectPackage.Ident("HTTPClient"),
		", baseURL string, opts ...", clientOption, ") ", names.Client, " {")
	if len(service.Methods) > 0 {
		g.P("baseURL = ", stringsPackage.Ident("TrimRight"), `(baseURL, "/")`)
	}
	g.P("return &", names.ClientImpl, "{")
	for _, method := range service.Methods {
		g.P(unexport(method.GoName), ": ",
			connectPackage.Ident("NewClient"),
			"[", method.Input.GoIdent, ", ", method.Output.GoIdent, "]",
			"(",
		)
		g.P("httpClient,")
		g.P(`baseURL + `, procedureConstName(method), `,`)
		g.P(connectPackage.Ident("WithSchema"), "(", procedureVarMethodDescriptor(method), "),")
		idempotency := methodIdempotency(method)
		switch idempotency {
		case connect.IdempotencyNoSideEffects:
			g.P(connectPackage.Ident("WithIdempotency"), "(", connectPackage.Ident("IdempotencyNoSideEffects"), "),")
		case connect.IdempotencyIdempotent:
			g.P(connectPackage.Ident("WithIdempotency"), "(", connectPackage.Ident("IdempotencyIdempotent"), "),")
		case connect.IdempotencyUnknown:
		}
		g.P(connectPackage.Ident("WithClientOptions"), "(opts...),")
		g.P("),")
	}
	g.P("}")
	g.P("}")
	g.P()

	// Client struct.
	wrapComments(g, names.ClientImpl, " implements ", names.Client, ".")
	g.P("type ", names.ClientImpl, " struct {")
	for _, method := range service.Methods {
		g.P(unexport(method.GoName), " *", connectPackage.Ident("Client"),
			"[", method.Input.GoIdent, ", ", method.Output.GoIdent, "]")
	}
	g.P("}")
	g.P()
	for _, method := range service.Methods {
		generateClientMethod(g, method, names)
	}
}

func generateClientMethod(g *protogen.GeneratedFile, method *protogen.Method, names names) {
	receiver := names.ClientImpl
	isStreamingClient := method.Desc.IsStreamingClient()
	isStreamingServer := method.Desc.IsStreamingServer()
	wrapComments(g, method.GoName, " calls ", method.Desc.FullName(), ".")
	if isDeprecatedMethod(method) {
		g.P("//")
		deprecated(g)
	}
	g.P("func (c *", receiver, ") ", clientSignature(g, method, true /* named */), " {")

	switch {
	case isStreamingClient && !isStreamingServer:
		g.P("return c.", unexport(method.GoName), ".CallClientStream(ctx)")
	case !isStreamingClient && isStreamingServer:
		// g.P("return c.", unexport(method.GoName), ".CallServerStream(ctx, req)")    //:::Modified:::
		g.P("return c.", unexport(method.GoName), ".CallServerStream(ctx, ",connectPackage.Ident("NewRequest"),"(req))")  // modified to wrap the request

	case isStreamingClient && isStreamingServer:
		g.P("return c.", unexport(method.GoName), ".CallBidiStream(ctx)")
	default:
		// g.P("return c.", unexport(method.GoName), ".CallUnary(ctx, req)")   //:::Modified:::
		g.P("res, err := c.", unexport(method.GoName), ".CallUnary(ctx, ",connectPackage.Ident("NewRequest"),"(req))")  // modified to wrap the request and response to message type
		g.P("if err != nil { return nil, err }")
		g.P("return res.Msg, nil")
	}
	g.P("}")
	g.P()
}

func clientSignature(g *protogen.GeneratedFile, method *protogen.Method, named bool) string {
	reqName := "req"
	ctxName := "ctx"
	if !named {
		reqName, ctxName = "", ""
	}
	if method.Desc.IsStreamingClient() && method.Desc.IsStreamingServer() {
		// bidi streaming
		return method.GoName + "(" + ctxName + " " + g.QualifiedGoIdent(contextPackage.Ident("Context")) + ") " +
			"*" + g.QualifiedGoIdent(connectPackage.Ident("BidiStreamForClient")) +
			"[" + g.QualifiedGoIdent(method.Input.GoIdent) + ", " + g.QualifiedGoIdent(method.Output.GoIdent) + "]"
	}
	if method.Desc.IsStreamingClient() {
		// client streaming
		return method.GoName + "(" + ctxName + " " + g.QualifiedGoIdent(contextPackage.Ident("Context")) + ") " +
			"*" + g.QualifiedGoIdent(connectPackage.Ident("ClientStreamForClient")) +
			"[" + g.QualifiedGoIdent(method.Input.GoIdent) + ", " + g.QualifiedGoIdent(method.Output.GoIdent) + "]"
	}
	if method.Desc.IsStreamingServer() {
		return method.GoName + "(" + ctxName + " " + g.QualifiedGoIdent(contextPackage.Ident("Context")) +
			", " + reqName + " *" + g.QualifiedGoIdent(method.Input.GoIdent) + ") " +  // this has changes to remove request generic  //:::Modified:::
			"(*" + g.QualifiedGoIdent(connectPackage.Ident("ServerStreamForClient")) +
			"[" + g.QualifiedGoIdent(method.Output.GoIdent) + "]" +
			", error)"
	}
	// unary; symmetric so we can re-use server templating
	return method.GoName + serverSignatureParams(g, method, named)
}

func generateServerInterface(g *protogen.GeneratedFile, service *protogen.Service, names names) {
	wrapComments(g, names.Server, " is an implementation of the ", service.Desc.FullName(), " service.")
	if isDeprecatedService(service) {
		g.P("//")
		deprecated(g)
	}
	g.AnnotateSymbol(names.Server, protogen.Annotation{Location: service.Location})
	g.P("type ", names.Server, " interface {")
	for _, method := range service.Methods {
		leadingComments(
			g,
			method.Comments.Leading,
			isDeprecatedMethod(method),
		)
		g.AnnotateSymbol(names.Server+"."+method.GoName, protogen.Annotation{Location: method.Location})
		g.P(serverSignature(g, method))
	}
	g.P("}")
	g.P()
}

func generateServerConstructor(g *protogen.GeneratedFile, service *protogen.Service, names names) {
	wrapComments(g, names.ServerConstructor, " builds an HTTP handler from the service implementation.",
		" It returns the path on which to mount the handler and the handler itself.")
	g.P("//")
	wrapComments(g, "By default, handlers support the Connect, gRPC, and gRPC-Web protocols with ",
		"the binary Protobuf and JSON codecs. They also support gzip compression.")
	if isDeprecatedService(service) {
		g.P("//")
		deprecated(g)
	}
	handlerOption := connectPackage.Ident("HandlerOption")
	g.P("func ", names.ServerConstructor, "(svc ", names.Server, ", opts ...", handlerOption,
		") (string, ", httpPackage.Ident("Handler"), ") {")
	for _, method := range service.Methods {
		isStreamingServer := method.Desc.IsStreamingServer()
		isStreamingClient := method.Desc.IsStreamingClient()
		idempotency := methodIdempotency(method)
		switch {
		case isStreamingClient && !isStreamingServer:
			g.P(procedureHandlerName(method), ` := `, connectPackage.Ident("NewClientStreamHandler"), "(")
		case !isStreamingClient && isStreamingServer:
			g.P(procedureHandlerName(method), ` := `, connectPackage.Ident("NewServerStreamHandler"), "(")
		case isStreamingClient && isStreamingServer:
			g.P(procedureHandlerName(method), ` := `, connectPackage.Ident("NewBidiStreamHandler"), "(")
		default:
			g.P(procedureHandlerName(method), ` := `, connectPackage.Ident("NewUnaryHandler"), "(")
		}
		g.P(procedureConstName(method), `,`)
		g.P("svc.", method.GoName, ",")
		g.P(connectPackage.Ident("WithSchema"), "(", procedureVarMethodDescriptor(method), "),")
		switch idempotency {
		case connect.IdempotencyNoSideEffects:
			g.P(connectPackage.Ident("WithIdempotency"), "(", connectPackage.Ident("IdempotencyNoSideEffects"), "),")
		case connect.IdempotencyIdempotent:
			g.P(connectPackage.Ident("WithIdempotency"), "(", connectPackage.Ident("IdempotencyIdempotent"), "),")
		case connect.IdempotencyUnknown:
		}
		g.P(connectPackage.Ident("WithHandlerOptions"), "(opts...),")
		g.P(")")
	}
	g.P(`return "/`, service.Desc.FullName(), `/", `, httpPackage.Ident("HandlerFunc"), `(func(w `, httpPackage.Ident("ResponseWriter"), `, r *`, httpPackage.Ident("Request"), `){`)
	g.P("switch r.URL.Path {")
	for _, method := range service.Methods {
		g.P("case ", procedureConstName(method), ":")
		g.P(procedureHandlerName(method), ".ServeHTTP(w, r)")
	}
	g.P("default:")
	g.P(httpPackage.Ident("NotFound"), "(w, r)")
	g.P("}")
	g.P("})")
	g.P("}")
	g.P()
}

func generateUnimplementedServerImplementation(g *protogen.GeneratedFile, service *protogen.Service, names names) {
	wrapComments(g, names.UnimplementedServer, " returns CodeUnimplemented from all methods.")
	g.P("type ", names.UnimplementedServer, " struct {}")
	g.P()
	for _, method := range service.Methods {
		g.P("func (", names.UnimplementedServer, ") ", serverSignature(g, method), "{")
		if method.Desc.IsStreamingServer() {
			g.P("return ", connectPackage.Ident("NewError"), "(",
				connectPackage.Ident("CodeUnimplemented"), ", ", errorsPackage.Ident("New"),
				`("`, method.Desc.FullName(), ` is not implemented"))`)
		} else {
			g.P("return nil, ", connectPackage.Ident("NewError"), "(",
				connectPackage.Ident("CodeUnimplemented"), ", ", errorsPackage.Ident("New"),
				`("`, method.Desc.FullName(), ` is not implemented"))`)
		}
		g.P("}")
		g.P()
	}
	g.P()
}

func serverSignature(g *protogen.GeneratedFile, method *protogen.Method) string {
	return method.GoName + serverSignatureParams(g, method, false /* named */)
}

func serverSignatureParams(g *protogen.GeneratedFile, method *protogen.Method, named bool) string {
	ctxName := "ctx "
	reqName := "req "
	streamName := "stream "
	if !named {
		ctxName, reqName, streamName = "", "", ""
	}
	if method.Desc.IsStreamingClient() && method.Desc.IsStreamingServer() {
		// bidi streaming
		return "(" + ctxName + g.QualifiedGoIdent(contextPackage.Ident("Context")) + ", " +
			streamName + "*" + g.QualifiedGoIdent(connectPackage.Ident("BidiStream")) +
			"[" + g.QualifiedGoIdent(method.Input.GoIdent) + ", " + g.QualifiedGoIdent(method.Output.GoIdent) + "]" +
			") error"
	}
	if method.Desc.IsStreamingClient() {
		// client streaming
		return "(" + ctxName + g.QualifiedGoIdent(contextPackage.Ident("Context")) + ", " +
			streamName + "*" + g.QualifiedGoIdent(connectPackage.Ident("ClientStream")) +
			"[" + g.QualifiedGoIdent(method.Input.GoIdent) + "]" +
			") (*" + g.QualifiedGoIdent(connectPackage.Ident("Response")) + "[" + g.QualifiedGoIdent(method.Output.GoIdent) + "] ,error)"
	}
	if method.Desc.IsStreamingServer() {
		// server streaming
		return "(" + ctxName + g.QualifiedGoIdent(contextPackage.Ident("Context")) +
			", " + reqName + "*" + g.QualifiedGoIdent(connectPackage.Ident("Request")) + "[" +
			g.QualifiedGoIdent(method.Input.GoIdent) + "], " +
			streamName + "*" + g.QualifiedGoIdent(connectPackage.Ident("ServerStream")) +
			"[" + g.QualifiedGoIdent(method.Output.GoIdent) + "]" +
			") error"
	}
	// unary
	return "(" + ctxName + g.QualifiedGoIdent(contextPackage.Ident("Context")) + // this has changes to remove request and response generic   //:::Modified:::
		", " + reqName + "*" +  g.QualifiedGoIdent(method.Input.GoIdent) + ") " +
		"(*" +  g.QualifiedGoIdent(method.Output.GoIdent) + ", error)"
}

func procedureConstName(m *protogen.Method) string {
	return fmt.Sprintf("%s%sProcedure", m.Parent.GoName, m.GoName)
}

func procedureHandlerName(m *protogen.Method) string {
	return fmt.Sprintf("%s%sHandler", unexport(m.Parent.GoName), m.GoName)
}

func procedureVarMethodDescriptor(m *protogen.Method) string {
	return unexport(fmt.Sprintf("%s%sMethodDescriptor", m.Parent.GoName, m.GoName))
}

func isDeprecatedService(service *protogen.Service) bool {
	serviceOptions, ok := service.Desc.Options().(*descriptorpb.ServiceOptions)
	return ok && serviceOptions.GetDeprecated()
}

func isDeprecatedMethod(method *protogen.Method) bool {
	methodOptions, ok := method.Desc.Options().(*descriptorpb.MethodOptions)
	return ok && methodOptions.GetDeprecated()
}

func methodIdempotency(method *protogen.Method) connect.IdempotencyLevel {
	methodOptions, ok := method.Desc.Options().(*descriptorpb.MethodOptions)
	if !ok {
		return connect.IdempotencyUnknown
	}
	switch methodOptions.GetIdempotencyLevel() {
	case descriptorpb.MethodOptions_NO_SIDE_EFFECTS:
		return connect.IdempotencyNoSideEffects
	case descriptorpb.MethodOptions_IDEMPOTENT:
		return connect.IdempotencyIdempotent
	case descriptorpb.MethodOptions_IDEMPOTENCY_UNKNOWN:
		return connect.IdempotencyUnknown
	}
	return connect.IdempotencyUnknown
}

// Raggedy comments in the generated code are driving me insane. This
// word-wrapping function is ruinously inefficient, but it gets the job done.
func wrapComments(g *protogen.GeneratedFile, elems ...any) {
	text := &bytes.Buffer{}
	for _, el := range elems {
		switch el := el.(type) {
		case protogen.GoIdent:
			fmt.Fprint(text, g.QualifiedGoIdent(el))
		default:
			fmt.Fprint(text, el)
		}
	}
	words := strings.Fields(text.String())
	text.Reset()
	var pos int
	for _, word := range words {
		numRunes := utf8.RuneCountInString(word)
		if pos > 0 && pos+numRunes+1 > commentWidth {
			g.P("// ", text.String())
			text.Reset()
			pos = 0
		}
		if pos > 0 {
			text.WriteRune(' ')
			pos++
		}
		text.WriteString(word)
		pos += numRunes
	}
	if text.Len() > 0 {
		g.P("// ", text.String())
	}
}

func leadingComments(g *protogen.GeneratedFile, comments protogen.Comments, isDeprecated bool) {
	if comments.String() != "" {
		g.P(strings.TrimSpace(comments.String()))
	}
	if isDeprecated {
		if comments.String() != "" {
			g.P("//")
		}
		deprecated(g)
	}
}

func deprecated(g *protogen.GeneratedFile) {
	g.P("// Deprecated: do not use.")
}

func unexport(s string) string {
	lowercased := strings.ToLower(s[:1]) + s[1:]
	switch lowercased {
	// https://go.dev/ref/spec#Keywords
	case "break", "default", "func", "interface", "select",
		"case", "defer", "go", "map", "struct",
		"chan", "else", "goto", "package", "switch",
		"const", "fallthrough", "if", "range", "type",
		"continue", "for", "import", "return", "var":
		return "_" + lowercased
	default:
		return lowercased
	}
}

type names struct {
	Base                string
	Client              string
	ClientConstructor   string
	ClientImpl          string
	ClientExposeMethod  string
	Server              string
	ServerConstructor   string
	UnimplementedServer string
}

func newNames(service *protogen.Service) names {
	base := service.GoName
	return names{
		Base:                base,
		Client:              fmt.Sprintf("%sClient", base),
		ClientConstructor:   fmt.Sprintf("New%sClient", base), // changes to lower new to make it internal   //:::Modified:::
		ClientImpl:          fmt.Sprintf("%sClient", unexport(base)),
		Server:              fmt.Sprintf("%sHandler", base),
		ServerConstructor:   fmt.Sprintf("New%sHandler", base),
		UnimplementedServer: fmt.Sprintf("Unimplemented%sHandler", base),
	}
}