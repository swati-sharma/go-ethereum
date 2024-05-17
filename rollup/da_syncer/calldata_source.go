package da_syncer

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/scroll-tech/go-ethereum/accounts/abi"
	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/rollup/types/encoding/codecv0"
)

var (
	callDataSourceFetchBlockRange uint64 = 100
)

type CalldataSource struct {
	ctx                           context.Context
	l1Client                      *L1Client
	l1height                      uint64
	maxL1Height                   uint64
	scrollChainABI                *abi.ABI
	l1CommitBatchEventSignature   common.Hash
	l1RevertBatchEventSignature   common.Hash
	l1FinalizeBatchEventSignature common.Hash
}

func NewCalldataSource(ctx context.Context, l1height, maxL1Height uint64, l1Client *L1Client) (DataSource, error) {
	scrollChainABI, err := scrollChainMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("failed to get scroll chain abi: %w", err)
	}
	return &CalldataSource{
		ctx:                           ctx,
		l1Client:                      l1Client,
		l1height:                      l1height,
		maxL1Height:                   maxL1Height,
		scrollChainABI:                scrollChainABI,
		l1CommitBatchEventSignature:   scrollChainABI.Events["CommitBatch"].ID,
		l1RevertBatchEventSignature:   scrollChainABI.Events["RevertBatch"].ID,
		l1FinalizeBatchEventSignature: scrollChainABI.Events["FinalizeBatch"].ID,
	}, nil
}

func (ds *CalldataSource) NextData() (DA, error) {
	to := ds.l1height + callDataSourceFetchBlockRange
	if to > ds.maxL1Height {
		to = ds.maxL1Height
	}
	if ds.l1height > to {
		return nil, sourceExhaustedErr
	}
	logs, err := ds.l1Client.fetchRollupEventsInRange(ds.ctx, ds.l1height, to)
	if err != nil {
		return nil, fmt.Errorf("cannot get events, l1height: %d, error: %v", ds.l1height, err)
	}
	return ds.processLogsToDA(logs)
}

func (ds *CalldataSource) L1Height() uint64 {
	return ds.l1height
}

func (ds *CalldataSource) processLogsToDA(logs []types.Log) (DA, error) {
	var da DA
	for _, vLog := range logs {
		switch vLog.Topics[0] {
		case ds.l1CommitBatchEventSignature:
			event := &L1CommitBatchEvent{}
			if err := UnpackLog(ds.scrollChainABI, event, "CommitBatch", vLog); err != nil {
				return nil, fmt.Errorf("failed to unpack commit rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()
			log.Trace("found new CommitBatch event", "batch index", batchIndex)

			daEntry, err := ds.getCommitBatchDa(batchIndex, &vLog)
			if err != nil {
				return nil, fmt.Errorf("failed to get commit batch da: %v, err: %w", batchIndex, err)
			}
			da = append(da, daEntry)

		case ds.l1RevertBatchEventSignature:
			event := &L1RevertBatchEvent{}
			if err := UnpackLog(ds.scrollChainABI, event, "RevertBatch", vLog); err != nil {
				return nil, fmt.Errorf("failed to unpack revert rollup event log, err: %w", err)
			}
			batchIndex := event.BatchIndex.Uint64()
			log.Trace("found new RevertBatch event", "batch index", batchIndex)
			da = append(da, NewRevertBatchDA(batchIndex))

		case ds.l1FinalizeBatchEventSignature:
			event := &L1FinalizeBatchEvent{}
			if err := UnpackLog(ds.scrollChainABI, event, "FinalizeBatch", vLog); err != nil {
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

func (ds *CalldataSource) getCommitBatchDa(batchIndex uint64, vLog *types.Log) (DAEntry, error) {
	var chunks Chunks
	if batchIndex == 0 {
		return NewCommitBatchDaV0(0, batchIndex, nil, []byte{}, chunks), nil
	}

	txData, err := ds.l1Client.fetchTxData(ds.ctx, vLog)
	if err != nil {
		return nil, err
	}
	const methodIDLength = 4
	if len(txData) < methodIDLength {
		return nil, fmt.Errorf("transaction data is too short, length of tx data: %v, minimum length required: %v", len(txData), methodIDLength)
	}

	method, err := ds.scrollChainABI.MethodById(txData[:methodIDLength])
	if err != nil {
		return nil, fmt.Errorf("failed to get method by ID, ID: %v, err: %w", txData[:methodIDLength], err)
	}

	values, err := method.Inputs.Unpack(txData[methodIDLength:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack transaction data using ABI, tx data: %v, err: %w", txData, err)
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
		return nil, fmt.Errorf("failed to decode calldata into commitBatch args, values: %+v, err: %w", values, err)
	}

	// todo: use codecv0 chunks
	chunks, err = decodeChunks(args.Chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack chunks: %v, err: %w", batchIndex, err)
	}
	parentBatchHeader, err := codecv0.NewDABatchFromBytes(args.ParentBatchHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode batch bytes into batch, values: %v, err: %w", args.ParentBatchHeader, err)
	}
	da := NewCommitBatchDaV0(args.Version, batchIndex, parentBatchHeader, args.SkippedL1MessageBitmap, chunks)
	return da, nil

	// parentTotalL1MessagePopped := getBatchTotalL1MessagePopped(args.ParentBatchHeader)
	// totalL1MessagePopped := countTotalL1MessagePopped(chunks)
	// skippedBitmap, err := decodeBitmap(args.SkippedL1MessageBitmap, totalL1MessagePopped)
	// if err != nil {
	// 	return nil, nil, fmt.Errorf("failed to decode bitmap: %v, err: %w", batchIndex, err)
	// }
	// // get all necessary l1msgs without skipped
	// currentIndex := parentTotalL1MessagePopped
	// for index := 0; index < int(totalL1MessagePopped); index++ {
	// 	for isL1MessageSkipped(skippedBitmap, currentIndex-parentTotalL1MessagePopped) {
	// 		currentIndex++
	// 	}
	// 	l1Tx := rawdb.ReadL1Message(ds.db, currentIndex)
	// 	if l1Tx == nil {
	// 		return nil, nil, fmt.Errorf("failed to read L1 message from db, l1 message index: %v", currentIndex)
	// 	}
	// 	l1Txs = append(l1Txs, l1Tx)
	// 	currentIndex++
	// }
	// return chunks, l1Txs, nil
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
