package lookup

import (
	"context"
	"fmt"
	"log"

	"connectrpc.com/connect"
	bingov1 "github.com/ericvolp12/bingo/gen/bingo/v1"
	"github.com/ericvolp12/bingo/pkg/store"
)

type Server struct {
	Store *store.Store
}

func NewServer(store *store.Store) *Server {
	return &Server{
		Store: store,
	}
}

func (s *Server) Lookup(
	ctx context.Context,
	req *connect.Request[bingov1.LookupRequest],
) (*connect.Response[bingov1.LookupResponse], error) {
	log.Println("Lookup called")
	if req.Msg.HandleOrDid == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("handle or did cannot be empty"))
	}

	entry, err := s.Store.Lookup(ctx, req.Msg.HandleOrDid)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	res := connect.NewResponse(&bingov1.LookupResponse{
		Handle:          entry.Handle,
		Did:             entry.Did,
		IsValid:         entry.IsValid,
		LastCheckedTime: entry.LastCheckedTime,
	})

	res.Header().Set("Bingo-Version", "v1")
	return res, nil
}

func (s *Server) BulkLookup(
	ctx context.Context,
	req *connect.Request[bingov1.BulkLookupRequest],
) (*connect.Response[bingov1.BulkLookupResponse], error) {
	log.Println("BulkLookup called")
	if len(req.Msg.HandlesOrDids) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("handles or dids cannot be empty"))
	}

	// Split the request into two slices, one for DIDs and one for handles.
	dids := []string{}
	handles := []string{}

	for _, handleOrDid := range req.Msg.HandlesOrDids {
		if store.IsDID(handleOrDid) {
			dids = append(dids, handleOrDid)
		} else {
			handles = append(handles, handleOrDid)
		}
	}

	// Lookup the DIDs and handles.
	responses := []*bingov1.LookupResponse{}
	if len(dids) > 0 {
		didEntries, err := s.Store.BulkLookupByDid(ctx, dids)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, entry := range didEntries {
			responses = append(responses, &bingov1.LookupResponse{
				Handle:          entry.Handle,
				Did:             entry.Did,
				IsValid:         entry.IsValid,
				LastCheckedTime: entry.LastCheckedTime,
			})
		}
	}
	if len(handles) > 0 {
		handleEntries, err := s.Store.BulkLookupByHandle(ctx, handles)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, entry := range handleEntries {
			responses = append(responses, &bingov1.LookupResponse{
				Handle:          entry.Handle,
				Did:             entry.Did,
				IsValid:         entry.IsValid,
				LastCheckedTime: entry.LastCheckedTime,
			})
		}
	}

	res := connect.NewResponse(&bingov1.BulkLookupResponse{
		Responses: responses,
	})

	res.Header().Set("Bingo-Version", "v1")
	return res, nil
}
