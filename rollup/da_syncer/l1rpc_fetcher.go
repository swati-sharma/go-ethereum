package da_syncer

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"reflect"

	"github.com/scroll-tech/go-ethereum/accounts/abi"
	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core/rawdb"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/ethdb"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/params"
	"github.com/scroll-tech/go-ethereum/rollup/sync_service"
)

type L1RPCFetcher struct {
	fetchBlockRange               uint64
	client                        *L1Client
	db                            ethdb.Database
	ctx                           context.Context
	latestProcessedBlock          uint64
	scrollChainABI                *abi.ABI
	l1CommitBatchEventSignature   common.Hash
	l1RevertBatchEventSignature   common.Hash
	l1FinalizeBatchEventSignature common.Hash
}

func newL1RpcDaFetcher(ctx context.Context, genesisConfig *params.ChainConfig, l1Client sync_service.EthClient, db ethdb.Database, l1DeploymentBlock, fetchBlockRange uint64) (DaFetcher, error) {
	// terminate if the caller does not provide an L1 client (e.g. in tests)
	if l1Client == nil || (reflect.ValueOf(l1Client).Kind() == reflect.Ptr && reflect.ValueOf(l1Client).IsNil()) {
		log.Warn("No L1 client provided, L1 rollup sync service will not run")
		return nil, nil
	}

	if genesisConfig.Scroll.L1Config == nil {
		return nil, fmt.Errorf("missing L1 config in genesis")
	}

	scrollChainABI, err := scrollChainMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get scroll chain abi: %w", err)
	}

	client, err := newL1Client(ctx, l1Client, genesisConfig.Scroll.L1Config.L1ChainId, genesisConfig.Scroll.L1Config.ScrollChainAddress, scrollChainABI)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize l1 client: %w", err)
	}

	// Initialize the latestProcessedBlock with the block just before the L1 deployment block.
	// This serves as a default value when there's no L1 rollup events synced in the database.
	var latestProcessedBlock uint64
	if l1DeploymentBlock > 0 {
		latestProcessedBlock = l1DeploymentBlock - 1
	}
	block := rawdb.ReadDASyncedL1BlockNumber(db)
	if block != nil {
		// restart from latest synced block number
		latestProcessedBlock = *block
	}

	daFetcher := L1RPCFetcher{
		fetchBlockRange:               fetchBlockRange,
		ctx:                           ctx,
		client:                        client,
		db:                            db,
		latestProcessedBlock:          latestProcessedBlock,
		scrollChainABI:                scrollChainABI,
		l1CommitBatchEventSignature:   scrollChainABI.Events["CommitBatch"].ID,
		l1RevertBatchEventSignature:   scrollChainABI.Events["RevertBatch"].ID,
		l1FinalizeBatchEventSignature: scrollChainABI.Events["FinalizeBatch"].ID,
	}
	return &daFetcher, nil
}

func (f *L1RPCFetcher) SetLatestProcessedBlock(to uint64) {
	f.latestProcessedBlock = to
}

// Fetch DA fetches all da events and converts it to DA format in some fetchBlockRange
func (f *L1RPCFetcher) FetchDA() (DA, uint64, error) {
	latestConfirmed, err := f.client.getLatestFinalizedBlockNumber(f.ctx)
	if err != nil {
		log.Warn("failed to get latest confirmed block number", "err", err)
		return nil, 0, err
	}

	log.Trace("Da fetcher fetch rollup events", "latest processed block", f.latestProcessedBlock, "latest confirmed", latestConfirmed)

	from := f.latestProcessedBlock + 1
	to := f.latestProcessedBlock + f.fetchBlockRange
	if to > latestConfirmed {
		to = latestConfirmed
	}

	logs, err := f.client.fetchRollupEventsInRange(f.ctx, from, to)
	if err != nil {
		log.Error("failed to fetch rollup events in range", "from block", from, "to block", to, "err", err)
		return nil, 0, err
	}
	da, err := f.processLogsToDA(logs)
	if err != nil {
		log.Error("failed to process rollup events in range", "from block", from, "to block", to, "err", err)
		return nil, 0, err
	}
	log.Trace("L1Rpcfetcher fetched and processed logs to da", "from", from, "to", to, "da len", len(da))
	return da, to, nil
}

func (f *L1RPCFetcher) processLogsToDA(logs []types.Log) (DA, error) {
	var da DA
	for _, vLog := range logs {
		switch vLog.Topics[0] {
		case f.l1CommitBatchEventSignature:
			event := &L1CommitBatchEvent{}
			if err := UnpackLog(f.scrollChainABI, event, "CommitBatch", vLog); err != nil {
				return nil, fmt.Errorf("failed to unpack commit rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()
			log.Trace("found new CommitBatch event", "batch index", batchIndex)

			chunks, l1Txs, err := f.getBatch(batchIndex, &vLog)
			if err != nil {
				return nil, fmt.Errorf("failed to get chunks, batch index: %v, err: %w", batchIndex, err)
			}
			da = append(da, NewCommitBatchDA(batchIndex, chunks, l1Txs))

		case f.l1RevertBatchEventSignature:
			event := &L1RevertBatchEvent{}
			if err := UnpackLog(f.scrollChainABI, event, "RevertBatch", vLog); err != nil {
				return nil, fmt.Errorf("failed to unpack revert rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()
			log.Trace("found new RevertBatch event", "batch index", batchIndex)
			da = append(da, NewRevertBatchDA(batchIndex))

		case f.l1FinalizeBatchEventSignature:
			event := &L1FinalizeBatchEvent{}
			if err := UnpackLog(f.scrollChainABI, event, "FinalizeBatch", vLog); err != nil {
				return nil, fmt.Errorf("failed to unpack finalized rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()
			log.Trace("found new FinalizeBatch event", "batch index", batchIndex)

			da = append(da, NewFinalizeBatchDA(batchIndex))

		default:
			return nil, fmt.Errorf("unknown event, topic: %v, tx hash: %v", vLog.Topics[0].Hex(), vLog.TxHash.Hex())
		}
	}
	return da, nil
}

func (f *L1RPCFetcher) getBatch(batchIndex uint64, vLog *types.Log) (Chunks, []*types.L1MessageTx, error) {
	var chunks Chunks
	var l1Txs []*types.L1MessageTx
	if batchIndex == 0 {
		return chunks, l1Txs, nil
	}

	tx, _, err := f.client.client.TransactionByHash(f.ctx, vLog.TxHash)
	if err != nil {
		log.Debug("failed to get transaction by hash, probably an unindexed transaction, fetching the whole block to get the transaction",
			"tx hash", vLog.TxHash.Hex(), "block number", vLog.BlockNumber, "block hash", vLog.BlockHash.Hex(), "err", err)
		block, err := f.client.client.BlockByHash(f.ctx, vLog.BlockHash)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get block by hash, block number: %v, block hash: %v, err: %w", vLog.BlockNumber, vLog.BlockHash.Hex(), err)
		}

		found := false
		for _, txInBlock := range block.Transactions() {
			if txInBlock.Hash() == vLog.TxHash {
				tx = txInBlock
				found = true
				break
			}
		}
		if !found {
			return nil, nil, fmt.Errorf("transaction not found in the block, tx hash: %v, block number: %v, block hash: %v", vLog.TxHash.Hex(), vLog.BlockNumber, vLog.BlockHash.Hex())
		}
	}

	txData := tx.Data()
	const methodIDLength = 4
	if len(txData) < methodIDLength {
		return nil, nil, fmt.Errorf("transaction data is too short, length of tx data: %v, minimum length required: %v", len(txData), methodIDLength)
	}

	method, err := f.scrollChainABI.MethodById(txData[:methodIDLength])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get method by ID, ID: %v, err: %w", txData[:methodIDLength], err)
	}

	values, err := method.Inputs.Unpack(txData[methodIDLength:])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unpack transaction data using ABI, tx data: %v, err: %w", txData, err)
	}

	type commitBatchArgs struct {
		Version                uint8
		ParentBatchHeader      []byte
		Chunks                 [][]byte
		SkippedL1MessageBitmap []byte
	}
	var args commitBatchArgs
	err = method.Inputs.Copy(&args, values)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode calldata into commitBatch args, values: %+v, err: %w", values, err)
	}

	chunks, err = decodeChunks(args.Chunks)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unpack chunks: %v, err: %w", batchIndex, err)
	}

	parentTotalL1MessagePopped := getBatchTotalL1MessagePopped(args.ParentBatchHeader)
	totalL1MessagePopped := countTotalL1MessagePopped(chunks)
	skippedBitmap, err := decodeBitmap(args.SkippedL1MessageBitmap, totalL1MessagePopped)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode bitmap: %v, err: %w", batchIndex, err)
	}

	// get all necessary l1msgs without skipped
	currentIndex := parentTotalL1MessagePopped
	for index := 0; index < int(totalL1MessagePopped); index++ {
		for isL1MessageSkipped(skippedBitmap, currentIndex-parentTotalL1MessagePopped) {
			currentIndex++
		}
		l1Tx := rawdb.ReadL1Message(f.db, currentIndex)
		if l1Tx == nil {
			return nil, nil, fmt.Errorf("failed to read L1 message from db, l1 message index: %v", currentIndex)
		}
		l1Txs = append(l1Txs, l1Tx)
		currentIndex++
	}
	return chunks, l1Txs, nil

}

func getBatchTotalL1MessagePopped(batchHeader []byte) uint64 {
	return binary.BigEndian.Uint64(batchHeader[17:25])
}

func decodeBitmap(skippedL1MessageBitmap []byte, totalL1MessagePopped uint64) ([]*big.Int, error) {
	length := len(skippedL1MessageBitmap)
	if length%32 != 0 {
		return nil, fmt.Errorf("skippedL1MessageBitmap length doesn't match, skippedL1MessageBitmap length should be equal 0 modulo 32, length of skippedL1MessageBitmap: %v", length)
	}
	if length*8 < int(totalL1MessagePopped) {
		return nil, fmt.Errorf("skippedL1MessageBitmap length is too small, skippedL1MessageBitmap length should be at least %v, length of skippedL1MessageBitmap: %v", (totalL1MessagePopped+7)/8, length)
	}
	var skippedBitmap []*big.Int
	for index := 0; index < length/32; index++ {
		bitmap := big.NewInt(0).SetBytes(skippedL1MessageBitmap[index*32 : index*32+32])
		skippedBitmap = append(skippedBitmap, bitmap)
	}
	return skippedBitmap, nil
}

func isL1MessageSkipped(skippedBitmap []*big.Int, index uint64) bool {
	quo := index / 256
	rem := index % 256
	return skippedBitmap[quo].Bit(int(rem)) != 0
}

// l2geth --scroll --datadir "./l2geth-datadir" --gcmode archive --cache.noprefetch --http --http.addr "0.0.0.0" --http.port 8545 --http.api "eth,net,web3,debug,scroll" --l1.endpoint "$L2GETH_L1_ENDPOINT" --da.sync
