package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	cometrpc "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"
)

type Client struct {
	Client *cometrpc.HTTP
}

var (
	clients    = make(map[string]*Client)
	clientsMux sync.RWMutex
)

func GetClient(rpcEndpoint string) (*Client, error) {
	clientsMux.RLock()
	if client, exists := clients[rpcEndpoint]; exists {
		clientsMux.RUnlock()
		return client, nil
	}
	clientsMux.RUnlock()

	// If client doesn't exist, acquire write lock and create it
	clientsMux.Lock()
	defer clientsMux.Unlock()

	// Double-check after acquiring write lock
	if client, exists := clients[rpcEndpoint]; exists {
		return client, nil
	}

	// Create new client
	cmtCli, err := cometrpc.New(rpcEndpoint, "/websocket")
	if err != nil {
		return nil, err
	}

	client := &Client{
		Client: cmtCli,
	}

	clients[rpcEndpoint] = client
	return client, nil
}

func (c *Client) BroadcastTx(txBytes []byte, waitForTx bool) (*coretypes.ResultBroadcastTx, error) {
	ctx := context.Background()

	t := tmtypes.Tx(txBytes)
	res, err := c.Client.BroadcastTxSync(ctx, t)
	if err != nil {
		return nil, err
	}

	if res.Code != 0 {
		return res, fmt.Errorf("broadcast error code %d: %s", res.Code, res.Log)
	}

	if waitForTx {
		_, err := c.WaitForTx(ctx, res.Hash.String())
		if err != nil {
			return nil, fmt.Errorf("failed to wait for transaction: %w", err)
		}
	}

	return res, nil
}

// WaitForTx requests the tx from hash, if not found, waits for next block and
// tries again. Returns an error if ctx is canceled.
func (c Client) WaitForTx(ctx context.Context, hash string) (*coretypes.ResultTx, error) {
	bz, err := hex.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("unable to decode tx hash '%s': %w", hash, err)
	}
	for {
		resp, err := c.Client.Tx(ctx, bz, false)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				// Tx not found, wait for next block and try again
				err := c.WaitForNextBlock(ctx)
				if err != nil {
					return nil, fmt.Errorf("waiting for next block: %w", err)
				}
				continue
			}
			return nil, fmt.Errorf("fetching tx '%s': %w", hash, err)
		}
		// Tx found
		return resp, nil
	}
}

// WaitForNextBlock waits until next block is committed.
// It reads the current block height and then waits for another block to be
// committed, or returns an error if ctx is canceled.
func (c Client) WaitForNextBlock(ctx context.Context) error {
	return c.WaitForNBlocks(ctx, 1)
}

// WaitForNBlocks reads the current block height and then waits for another n
// blocks to be committed, or returns an error if ctx is canceled.
func (c Client) WaitForNBlocks(ctx context.Context, n int64) error {
	start, err := c.LatestBlockHeight(ctx)
	if err != nil {
		return err
	}
	return c.WaitForBlockHeight(ctx, start+n)
}

// LatestBlockHeight returns the latest block height of the app.
func (c Client) LatestBlockHeight(ctx context.Context) (int64, error) {
	resp, err := c.Client.Status(ctx)
	if err != nil {
		return 0, err
	}
	return resp.SyncInfo.LatestBlockHeight, nil
}

// WaitForBlockHeight waits until block height h is committed, or returns an
// error if ctx is canceled.
func (c Client) WaitForBlockHeight(ctx context.Context, h int64) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		latestHeight, err := c.LatestBlockHeight(ctx)
		if err != nil {
			return err
		}
		if latestHeight >= h {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout exceeded waiting for block: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
