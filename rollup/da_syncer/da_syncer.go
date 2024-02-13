package da_syncer

import (
	"context"
	"math/big"
	"time"

	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/ethdb"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/params"
	"github.com/scroll-tech/go-ethereum/rollup/sync_service"
)

// Config is the configuration parameters of da syncer.
type Config struct {
	FetcherMode      FetcherMode // mode of fetcher
	SnapshotFilePath string      // path to snapshot file
}

// defaultSyncInterval is the frequency at which we query for new rollup event.
const defaultSyncInterval = 60 * time.Second

// defaultFetchBlockRange is number of L1 blocks that is loaded by fetcher in one request.
const defaultFetchBlockRange = 100

type DaSyncer struct {
	DaFetcher DaFetcher
	ctx       context.Context
	cancel    context.CancelFunc
	// batches is map from batchIndex to batch blocks
	batches map[uint64][]*types.Block
}

func NewDaSyncer(ctx context.Context, genesisConfig *params.ChainConfig, db ethdb.Database, l1Client sync_service.EthClient, l1DeploymentBlock uint64, config Config) (*DaSyncer, error) {
	ctx, cancel := context.WithCancel(ctx)
	var daFetcher DaFetcher
	var err error
	if config.FetcherMode == L1RPC {
		daFetcher, err = newL1RpcDaFetcher(ctx, genesisConfig, l1Client, db, l1DeploymentBlock, defaultFetchBlockRange)
		if err != nil {
			cancel()
			return nil, err
		}
	} else {
		daFetcher, err = newSnapshotFetcher(defaultFetchBlockRange)
		if err != nil {
			cancel()
			return nil, err
		}
	}
	daSyncer := DaSyncer{
		DaFetcher: daFetcher,
		ctx:       ctx,
		cancel:    cancel,
		batches:   make(map[uint64][]*types.Block),
	}
	return &daSyncer, nil
}

func (s *DaSyncer) Start() {
	if s == nil {
		return
	}

	log.Info("Starting DaSyncer")

	go func() {
		syncTicker := time.NewTicker(defaultSyncInterval)
		defer syncTicker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-syncTicker.C:
				s.syncWithDa()
			}
		}
	}()
}

func (s *DaSyncer) Stop() {
	if s == nil {
		return
	}

	log.Info("Stopping DaSyncer")

	if s.cancel != nil {
		s.cancel()
	}
}

func (s *DaSyncer) syncWithDa() {
	da, err := s.DaFetcher.FetchDA()
	if err != nil {
		log.Error("failed to fetch DA", "err", err)
		return
	}
	for _, daEntry := range da {
		switch daEntry.DaType {
		case CommitBatch:
			blocks, err := s.processDaToBlocks(daEntry)
			if err != nil {
				log.Warn("failed to process DA to blocks", "err", err)
				return
			}
			s.batches[daEntry.BatchIndex] = blocks
		case RevertBatch:
			delete(s.batches, daEntry.BatchIndex)
		case FinalizeBatch:
			blocks, ok := s.batches[daEntry.BatchIndex]
			if !ok {
				log.Warn("cannot find blocks for batch", "batch index", daEntry.BatchIndex, "err", err)
				return
			}
			s.insertBlocks(blocks)
		}
	}

}

func (s *DaSyncer) processDaToBlocks(daEntry *DAEntry) ([]*types.Block, error) {
	var blocks []*types.Block
	l1TxIndex := 0
	for _, chunk := range daEntry.Chunks {
		l2TxIndex := 0
		for _, blockContext := range chunk.BlockContexts {
			// create header
			header := types.Header{
				// todo: maybe need to get ParentHash here too
				Number:   big.NewInt(0).SetUint64(blockContext.BlockNumber),
				Time:     blockContext.Timestamp,
				BaseFee:  blockContext.BaseFee,
				GasLimit: blockContext.GasLimit,
			}
			// create txs
			var txs types.Transactions
			// insert l1 msgs
			for id := 0; id < int(blockContext.NumL1Messages); id++ {
				l1Tx := types.NewTx(daEntry.L1Txs[l1TxIndex])
				txs = append(txs, l1Tx)
				l1TxIndex++
			}
			// insert l2 txs
			for id := int(blockContext.NumL1Messages); id < int(blockContext.NumTransactions); id++ {
				var l2Tx *types.Transaction
				l2Tx.UnmarshalBinary(chunk.L2Txs[l2TxIndex])
				txs = append(txs, l2Tx)
				l2TxIndex++
			}
			block := types.NewBlockWithHeader(&header).WithBody(txs, make([]*types.Header, 0))
			blocks = append(blocks, block)
		}
	}
	return blocks, nil
}

func (s *DaSyncer) insertBlocks([]*types.Block) {

}
