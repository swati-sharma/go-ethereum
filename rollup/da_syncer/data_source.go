package da_syncer

import (
	"context"
	"errors"
	"math/big"

	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/params"
)

var (
	sourceExhaustedErr = errors.New("data source has been exhausted")
)

type DataSource interface {
	NextData() (DA, error)
	L1Height() uint64
}

type DataSourceFactory struct {
	config        Config
	genesisConfig *params.ChainConfig
	l1Client      *L1Client
}

func NewDataSourceFactory(blockchain *core.BlockChain, genesisConfig *params.ChainConfig, config Config, l1Client *L1Client) *DataSourceFactory {
	return &DataSourceFactory{
		config:        config,
		genesisConfig: genesisConfig,
		l1Client:      l1Client,
	}
}

func (ds *DataSourceFactory) OpenDataSource(ctx context.Context, l1height uint64) (DataSource, error) {
	if ds.config.FetcherMode == L1RPC {
		if ds.genesisConfig.IsBernoulli(big.NewInt(0).SetUint64(l1height)) {
			return nil, errors.New("blob_data_source: not implemented")
		} else {
			var maxL1Height uint64 = ds.genesisConfig.BernoulliBlock.Uint64()
			return NewCalldataSource(ctx, l1height, maxL1Height, ds.l1Client)
		}
	} else {
		return nil, errors.New("snapshot_data_source: not implemented")
	}
}
