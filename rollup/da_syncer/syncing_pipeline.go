package da_syncer

import (
	"context"
	"errors"
	"time"

	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/ethdb"
	"github.com/scroll-tech/go-ethereum/params"
	"github.com/scroll-tech/go-ethereum/rollup/sync_service"
)

// Config is the configuration parameters of data availability syncing.
type Config struct {
	FetcherMode      FetcherMode // mode of fetcher
	SnapshotFilePath string      // path to snapshot file
}

var (
	errInvalidChain = errors.New("retrieved hash chain is invalid")
)

// defaultSyncInterval is the frequency at which we query for new rollup event.
const defaultSyncInterval = 45 * time.Second

type SyncingPipeline struct {
	db         ethdb.Database
	blockchain *core.BlockChain
	blockQueue *BlockQueue
	daSyncer   *DaSyncer
}

func NewSyncingPipeline(ctx context.Context, blockchain *core.BlockChain, genesisConfig *params.ChainConfig, db ethdb.Database, ethClient sync_service.EthClient, l1DeploymentBlock uint64, config Config) (*SyncingPipeline, error) {
	var err error

	l1Client, err := newL1Client(ctx, genesisConfig, ethClient)
	if err != nil {
		return nil, err
	}

	dataSourceFactory := NewDataSourceFactory(blockchain, genesisConfig, config, l1Client)
	// todo: keep synced l1 height somewhere
	var syncedL1Height uint64 = 0
	daQueue := NewDaQueue(syncedL1Height, dataSourceFactory)
	batchQueue := NewBatchQueue(daQueue)
	blockQueue := NewBlockQueue(batchQueue)
	daSyncer := NewDaSyncer(blockchain)

	return &SyncingPipeline{
		db:         db,
		blockchain: blockchain,
		blockQueue: blockQueue,
		daSyncer:   daSyncer,
	}, nil
}

func (sp *SyncingPipeline) Step(ctx context.Context) error {
	block, err := sp.blockQueue.NextBlock(ctx)
	if err != nil {
		return err
	}
	err = sp.daSyncer.SyncOneBlock(block)
	return err
}

// func (s *DaSyncer) Start() {
// 	if s == nil {
// 		return
// 	}

// 	log.Info("Starting DaSyncer")

// 	go func() {
// 		syncTicker := time.NewTicker(defaultSyncInterval)
// 		defer syncTicker.Stop()

// 		for {
// 			s.syncWithDa()
// 			select {
// 			case <-s.ctx.Done():
// 				return
// 			case <-syncTicker.C:
// 				continue
// 			}
// 		}
// 	}()
// }

// func (s *DaSyncer) Stop() {
// 	if s == nil {
// 		return
// 	}

// 	log.Info("Stopping DaSyncer")

// 	if s.cancel != nil {
// 		s.cancel()
// 	}
// }

// func (s *DaSyncer) syncWithDa() {
// 	log.Info("DaSyncer syncing")
// 	da, to, err := s.DaFetcher.FetchDA()
// 	if err != nil {
// 		log.Error("failed to fetch DA", "err", err)
// 		return
// 	}
// 	for _, daEntry := range da {
// 		switch daEntry.DaType {
// 		case CommitBatch:
// 			blocks, err := s.processDaToBlocks(daEntry)
// 			if err != nil {
// 				log.Warn("failed to process DA to blocks", "err", err)
// 				return
// 			}
// 			log.Debug("commit batch", "batchindex", daEntry.BatchIndex)
// 			s.batches[daEntry.BatchIndex] = blocks
// 		case RevertBatch:
// 			log.Debug("revert batch", "batchindex", daEntry.BatchIndex)
// 			delete(s.batches, daEntry.BatchIndex)
// 		case FinalizeBatch:
// 			log.Debug("finalize batch", "batchindex", daEntry.BatchIndex)
// 			blocks, ok := s.batches[daEntry.BatchIndex]
// 			if !ok {
// 				log.Warn("cannot find blocks for batch", "batch index", daEntry.BatchIndex, "err", err)
// 				return
// 			}
// 			err := s.insertBlocks(blocks)
// 			if err != nil {
// 				log.Warn("cannot insert blocks for batch", "batch index", daEntry.BatchIndex, "err", err)
// 				return
// 			}
// 		}
// 	}
// 	rawdb.WriteDASyncedL1BlockNumber(s.db, to)
// 	s.DaFetcher.SetLatestProcessedBlock(to)
// 	log.Info("DaSyncer synced")
// }
