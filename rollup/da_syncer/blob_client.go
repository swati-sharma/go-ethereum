package da_syncer

import (
	"context"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/crypto/kzg4844"
	"github.com/scroll-tech/go-ethereum/params"

	"github.com/scroll-tech/go-ethereum/rollup/sync_service"
)

// BlobClient is a wrapper around EthClient that adds
// methods for conveniently collecting rollup events of ScrollChain contract.
type BlobClient struct {
	scrollChainAddress            common.Address
	l1CommitBatchEventSignature   common.Hash
	l1RevertBatchEventSignature   common.Hash
	l1FinalizeBatchEventSignature common.Hash
}

// newL1Client initializes a new L1Client instance with the provided configuration.
// It checks for a valid scrollChainAddress and verifies the chain ID.
func newBlobClient(ctx context.Context, genesisConfig *params.ChainConfig, l1Client sync_service.EthClient) (*BlobClient, error) {
	client := BlobClient{}

	return &client, nil
}

// fetchBlob fetches blob by it's commitment
func (c *BlobClient) fetchBlob(ctx context.Context, commitment []*kzg4844.Commitment) ([]*kzg4844.Blob, error) {
	// todo:
	return nil, nil
}
