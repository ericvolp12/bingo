// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: bingo/v1/bingo.proto

package bingov1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/ericvolp12/bingo/gen/bingo/v1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion0_1_0

const (
	// BingoServiceName is the fully-qualified name of the BingoService service.
	BingoServiceName = "bingo.v1.BingoService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// BingoServiceLookupProcedure is the fully-qualified name of the BingoService's Lookup RPC.
	BingoServiceLookupProcedure = "/bingo.v1.BingoService/Lookup"
)

// BingoServiceClient is a client for the bingo.v1.BingoService service.
type BingoServiceClient interface {
	Lookup(context.Context, *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error)
}

// NewBingoServiceClient constructs a client for the bingo.v1.BingoService service. By default, it
// uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and sends
// uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewBingoServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) BingoServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &bingoServiceClient{
		lookup: connect.NewClient[v1.LookupRequest, v1.LookupResponse](
			httpClient,
			baseURL+BingoServiceLookupProcedure,
			opts...,
		),
	}
}

// bingoServiceClient implements BingoServiceClient.
type bingoServiceClient struct {
	lookup *connect.Client[v1.LookupRequest, v1.LookupResponse]
}

// Lookup calls bingo.v1.BingoService.Lookup.
func (c *bingoServiceClient) Lookup(ctx context.Context, req *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error) {
	return c.lookup.CallUnary(ctx, req)
}

// BingoServiceHandler is an implementation of the bingo.v1.BingoService service.
type BingoServiceHandler interface {
	Lookup(context.Context, *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error)
}

// NewBingoServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewBingoServiceHandler(svc BingoServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	bingoServiceLookupHandler := connect.NewUnaryHandler(
		BingoServiceLookupProcedure,
		svc.Lookup,
		opts...,
	)
	return "/bingo.v1.BingoService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case BingoServiceLookupProcedure:
			bingoServiceLookupHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedBingoServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedBingoServiceHandler struct{}

func (UnimplementedBingoServiceHandler) Lookup(context.Context, *connect.Request[v1.LookupRequest]) (*connect.Response[v1.LookupResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("bingo.v1.BingoService.Lookup is not implemented"))
}