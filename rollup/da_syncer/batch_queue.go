package da_syncer

import (
	"context"
	"fmt"
)

type BatchQueue struct {
	// batches is map from batchIndex to batch blocks
	batches map[uint64]DAEntry
	daQueue *DaQueue
}

func NewBatchQueue(daQueue *DaQueue) *BatchQueue {
	return &BatchQueue{
		batches: make(map[uint64]DAEntry),
		daQueue: daQueue,
	}
}

func (bq *BatchQueue) NextBatch(ctx context.Context) (DAEntry, error) {

	for {
		daEntry, err := bq.daQueue.NextDA(ctx)
		if err != nil {
			return nil, err
		}
		switch daEntry := daEntry.(type) {
		case *CommitBatchDaV0:
			bq.batches[daEntry.BatchIndex] = daEntry
		case *RevertBatchDA:
			delete(bq.batches, daEntry.BatchIndex)
		case *FinalizeBatchDA:
			ret, ok := bq.batches[daEntry.BatchIndex]
			if !ok {
				return nil, fmt.Errorf("failed to get batch data, batchIndex: %d", daEntry.BatchIndex)
			}
			return ret, nil
		default:
			return nil, fmt.Errorf("unexpected type of daEntry: %T", daEntry)
		}
	}
}
