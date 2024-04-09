package da_syncer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/core/rawdb"
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

var (
	errInvalidChain = errors.New("retrieved hash chain is invalid")
)

// defaultSyncInterval is the frequency at which we query for new rollup event.
const defaultSyncInterval = 45 * time.Second

// defaultFetchBlockRange is number of L1 blocks that is loaded by fetcher in one request.
const defaultFetchBlockRange = 1000

type DaSyncer struct {
	DaFetcher  DaFetcher
	ctx        context.Context
	cancel     context.CancelFunc
	db         ethdb.Database
	blockchain *core.BlockChain
	// batches is map from batchIndex to batch blocks
	batches map[uint64][]*types.Block
}

func NewDaSyncer(ctx context.Context, blockchain *core.BlockChain, genesisConfig *params.ChainConfig, db ethdb.Database, l1Client sync_service.EthClient, l1DeploymentBlock uint64, config Config) (*DaSyncer, error) {
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
		DaFetcher:  daFetcher,
		ctx:        ctx,
		cancel:     cancel,
		db:         db,
		blockchain: blockchain,
		batches:    make(map[uint64][]*types.Block),
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
			s.syncWithDa()
			select {
			case <-s.ctx.Done():
				return
			case <-syncTicker.C:
				continue
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
	log.Info("DaSyncer syncing")
	da, to, err := s.DaFetcher.FetchDA()
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
			log.Info("commit batch", "batchindex", daEntry.BatchIndex)
			s.batches[daEntry.BatchIndex] = blocks
		case RevertBatch:
			log.Info("revert batch", "batchindex", daEntry.BatchIndex)
			delete(s.batches, daEntry.BatchIndex)
		case FinalizeBatch:
			log.Info("finalize batch", "batchindex", daEntry.BatchIndex)
			blocks, ok := s.batches[daEntry.BatchIndex]
			if !ok {
				log.Info("cannot find blocks for batch", "batch index", daEntry.BatchIndex, "err", err)
				return
			}
			s.insertBlocks(blocks)
		}
	}
	rawdb.WriteDASyncedL1BlockNumber(s.db, to)
}

func (s *DaSyncer) processDaToBlocks(daEntry *DAEntry) ([]*types.Block, error) {
	var blocks []*types.Block
	l1TxIndex := 0
	prevHash := s.blockchain.CurrentBlock().Hash()
	for _, chunk := range daEntry.Chunks {
		l2TxIndex := 0
		for _, blockContext := range chunk.BlockContexts {
			// create header
			header := types.Header{
				// todo: maybe need to get ParentHash here too
				ParentHash: prevHash,
				Number:     big.NewInt(0).SetUint64(blockContext.BlockNumber),
				Time:       blockContext.Timestamp,
				BaseFee:    blockContext.BaseFee,
				GasLimit:   blockContext.GasLimit,
			}
			// create txs
			// var txs types.Transactions
			txs := make(types.Transactions, 0, blockContext.NumTransactions)
			// insert l1 msgs
			for id := 0; id < int(blockContext.NumL1Messages); id++ {
				l1Tx := types.NewTx(daEntry.L1Txs[l1TxIndex])
				txs = append(txs, l1Tx)
				l1TxIndex++
			}
			// log.Info("processing block", "block hash", blockContext.BlockNumber, "numl1messages", blockContext.NumL1Messages, "numtxs", blockContext.NumTransactions)
			// insert l2 txs
			for id := int(blockContext.NumL1Messages); id < int(blockContext.NumTransactions); id++ {
				l2Tx := &types.Transaction{}
				// log.Info("processing l2 tx", "num", id, "l2tx", l2Tx)
				l2Tx.UnmarshalBinary(chunk.L2Txs[l2TxIndex])
				txs = append(txs, l2Tx)
				l2TxIndex++
			}
			block := types.NewBlockWithHeader(&header).WithBody(txs, make([]*types.Header, 0))
			prevHash = block.Hash()
			blocks = append(blocks, block)
		}
	}
	return blocks, nil
}

func (s *DaSyncer) insertBlocks(blocks []*types.Block) error {
	for _, block := range blocks {
		log.Info("block info", "number", block.Number(), "hash", block.Hash(), "parentHash", block.ParentHash())
		log.Info("block header", "block header", block.Header())
	}
	if index, err := s.blockchain.InsertChain(blocks); err != nil {
		log.Info("err != nil")
		if index < len(blocks) {
			log.Debug("Downloaded item processing failed", "number", blocks[index].Header().Number, "hash", blocks[index].Header().Hash(), "err", err)
		} else {
			// The InsertChain method in blockchain.go will sometimes return an out-of-bounds index,
			// when it needs to preprocess blocks to import a sidechain.
			// The importer will put together a new list of blocks to import, which is a superset
			// of the blocks delivered from the downloader, and the indexing will be off.
			log.Debug("Downloaded item processing failed on sidechain import", "index", index, "err", err)
		}
		return fmt.Errorf("%w: %v", errInvalidChain, err)
	}
	log.Info("insertblocks completed", "blockchain height", s.blockchain.CurrentBlock().Header().Number, "block hash", s.blockchain.CurrentBlock().Header().Hash())
	return nil
}
