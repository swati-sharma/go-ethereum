package da_syncer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/core/rawdb"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/ethdb"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/params"
	"github.com/scroll-tech/go-ethereum/rollup/sync_service"
	"github.com/scroll-tech/go-ethereum/trie"
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
			log.Debug("commit batch", "batchindex", daEntry.BatchIndex)
			s.batches[daEntry.BatchIndex] = blocks
		case RevertBatch:
			log.Debug("revert batch", "batchindex", daEntry.BatchIndex)
			delete(s.batches, daEntry.BatchIndex)
		case FinalizeBatch:
			log.Debug("finalize batch", "batchindex", daEntry.BatchIndex)
			blocks, ok := s.batches[daEntry.BatchIndex]
			if !ok {
				log.Warn("cannot find blocks for batch", "batch index", daEntry.BatchIndex, "err", err)
				return
			}
			s.insertBlocks(blocks)
		}
	}
	rawdb.WriteDASyncedL1BlockNumber(s.db, to)
	s.DaFetcher.SetLatestProcessedBlock(to)
}

func (s *DaSyncer) processDaToBlocks(daEntry *DAEntry) ([]*types.Block, error) {
	var blocks []*types.Block
	l1TxIndex := 0
	for _, chunk := range daEntry.Chunks {
		l2TxIndex := 0
		for _, blockContext := range chunk.BlockContexts {
			// create header
			header := types.Header{
				Number:   big.NewInt(0).SetUint64(blockContext.BlockNumber),
				Time:     blockContext.Timestamp,
				BaseFee:  blockContext.BaseFee,
				GasLimit: blockContext.GasLimit,
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
			// insert l2 txs
			for id := int(blockContext.NumL1Messages); id < int(blockContext.NumTransactions); id++ {
				l2Tx := &types.Transaction{}
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

func (s *DaSyncer) insertBlocks(blocks []*types.Block) error {
	prevHash := s.blockchain.CurrentBlock().Hash()
	for _, block := range blocks {
		header := block.Header()
		txs := block.Transactions()

		// fill header with all necessary fields
		var err error
		header.ReceiptHash, header.Bloom, header.Root, header.GasUsed, err = s.blockchain.PreprocessBlock(block)
		if err != nil {
			return fmt.Errorf("block preprocessing failed, block number: %d, error: %v", block.Number(), err)
		}
		header.UncleHash = common.HexToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347")
		header.Difficulty = common.Big1
		header.BaseFee = nil
		header.TxHash = types.DeriveSha(txs, trie.NewStackTrie(nil))
		header.ParentHash = prevHash

		fullBlock := types.NewBlockWithHeader(header).WithBody(txs, make([]*types.Header, 0))

		if index, err := s.blockchain.InsertChainWithoutSealVerification(fullBlock); err != nil {
			if index < len(blocks) {
				log.Debug("Block insert failed", "number", blocks[index].Header().Number, "hash", blocks[index].Header().Hash(), "err", err)
			}
			return fmt.Errorf("cannot insert block, number: %d, error: %v", block.Number(), err)
		}
		prevHash = fullBlock.Hash()
		log.Info("inserted block", "blockhain height", s.blockchain.CurrentBlock().Header().Number, "block hash", s.blockchain.CurrentBlock().Header().Hash())
	}

	log.Info("insertblocks completed", "blockchain height", s.blockchain.CurrentBlock().Header().Number, "block hash", s.blockchain.CurrentBlock().Header().Hash())
	return nil
}