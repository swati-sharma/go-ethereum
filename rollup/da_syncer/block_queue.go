package da_syncer

import (
	"context"
	"fmt"

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
		// to be implemented in codecv0
		// bq.blocks := codecv0.DecodeFromCalldata(daEntry)
	default:
		return fmt.Errorf("unexpected type of daEntry: %T", daEntry)
	}
	return nil
}
/*

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
*/