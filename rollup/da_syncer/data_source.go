package da_syncer

import (
	"context"
	"errors"

	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/ethdb"
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
	db            ethdb.Database
}

func NewDataSourceFactory(blockchain *core.BlockChain, genesisConfig *params.ChainConfig, config Config, l1Client *L1Client, db ethdb.Database) *DataSourceFactory {
	return &DataSourceFactory{
		config:        config,
		genesisConfig: genesisConfig,
		l1Client:      l1Client,
		db:            db,
	}
}

func (ds *DataSourceFactory) OpenDataSource(ctx context.Context, l1height uint64) (DataSource, error) {
	if ds.config.FetcherMode == L1RPC {
		if isBernoulliByL1Height(l1height) {
			return nil, errors.New("blob_data_source: not implemented")
		} else {
			// todo: set l1 block where l2 changes to bernoulli
			var maxL1Height uint64 = 1000000000000
			return NewCalldataSource(ctx, l1height, maxL1Height, ds.l1Client, ds.db)
		}
	} else {
		return nil, errors.New("snapshot_data_source: not implemented")
	}
}

func isBernoulliByL1Height(l1height uint64) bool {
	return false
}
