package da_syncer

import (
	"context"
	"fmt"
	"math/big"

	"github.com/scroll-tech/go-ethereum/core/types"
)

type BlockQueue struct {
	batchQueue *BatchQueue
	blocks     []*types.Block
}

func NewBlockQueue(batchQueue *BatchQueue) *BlockQueue {
	return &BlockQueue{
		batchQueue: batchQueue,
		blocks:     []*types.Block{},
	}
}

func (bq *BlockQueue) NextBlock(ctx context.Context) (*types.Block, error) {
	for len(bq.blocks) == 0 {
		err := bq.getBlocksFromBatch(ctx)
		if err != nil {
			return nil, err
		}
	}
	block := bq.blocks[0]
	bq.blocks = bq.blocks[1:]
	return block, nil
}

func (bq *BlockQueue) getBlocksFromBatch(ctx context.Context) error {
	daEntry, err := bq.batchQueue.NextBatch(ctx)
	if err != nil {
		return err
	}
	switch daEntry := daEntry.(type) {
	case *CommitBatchDaV0:
		bq.blocks, err = bq.processDaV0ToBlocks(daEntry)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected type of daEntry: %T", daEntry)
	}
	return nil
}

func (bq *BlockQueue) processDaV0ToBlocks(daEntry *CommitBatchDaV0) ([]*types.Block, error) {
	var blocks []*types.Block
	l1TxIndex := 0
	for _, chunk := range daEntry.Chunks {
		for blockId, daBlock := range chunk.Blocks {
			// create header
			header := types.Header{
				Number:   big.NewInt(0).SetUint64(daBlock.BlockNumber),
				Time:     daBlock.Timestamp,
				BaseFee:  daBlock.BaseFee,
				GasLimit: daBlock.GasLimit,
			}
			// create txs
			// var txs types.Transactions
			txs := make(types.Transactions, 0, daBlock.NumTransactions)
			// insert l1 msgs
			for id := 0; id < int(daBlock.NumL1Messages); id++ {
				l1Tx := types.NewTx(daEntry.L1Txs[l1TxIndex])
				txs = append(txs, l1Tx)
				l1TxIndex++
			}
			// insert l2 txs
			txs = append(txs, chunk.Transactions[blockId]...)
			block := types.NewBlockWithHeader(&header).WithBody(txs, make([]*types.Header, 0))
			blocks = append(blocks, block)
		}
	}
	return blocks, nil
}
