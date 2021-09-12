package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	jsonrpc "github.com/filecoin-project/go-jsonrpc"
	lotusapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
)

// LotusClient returns a JSONRPC client for the Lotus API
func LotusClient(ctx context.Context) (lotusapi.FullNode, jsonrpc.ClientCloser, error) {
	var headers map[string][]string
	if authToken := os.Getenv("LOTUS_TOKEN"); authToken != "" {
		headers = http.Header{"Authorization": []string{"Bearer " + authToken}}
	} else {
		return nil, nil, fmt.Errorf("LOTUS_TOKEN env var not set")
	}
	if addr := os.Getenv("LOTUS_API"); addr != "" {
		return client.NewFullNodeRPCV1(ctx, "ws://"+addr+"/rpc/v1", headers)
	} else {
		return nil, nil, fmt.Errorf("LOTUS_API env var not set")
	}
}
