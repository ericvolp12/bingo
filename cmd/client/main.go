package main

import (
	"context"
	"log"
	"net/http"

	// generated by protoc-gen-go
	bingov1 "github.com/ericvolp12/bingo/gen/bingo/v1"
	"github.com/ericvolp12/bingo/gen/bingo/v1/bingov1connect"

	"connectrpc.com/connect"
)

func main() {
	client := bingov1connect.NewBingoServiceClient(
		http.DefaultClient,
		"http://localhost:8923",
		connect.WithGRPC(),
	)
	res, err := client.Lookup(
		context.Background(),
		connect.NewRequest(&bingov1.LookupRequest{HandleOrDid: "bingo"}),
	)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(res.Msg.Handle)
}
