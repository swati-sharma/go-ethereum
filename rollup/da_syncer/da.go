package da_syncer

import (
	"github.com/scroll-tech/go-ethereum/rollup/types/encoding/codecv0"
)

type DAType int

const (
	// CommitBatch contains data of event of CommitBatch
	CommitBatchV0 DAType = iota
	// RevertBatch contains data of event of RevertBatch
	RevertBatch
	// FinalizeBatch contains data of event of FinalizeBatch
	FinalizeBatch
)

type DAEntry interface {
	DAType() DAType
}

type DA []DAEntry

type CommitBatchDaV0 struct {
	DaType                 DAType
	Version                uint8
	BatchIndex             uint64
	ParentBatchHeader      *codecv0.DABatch
	SkippedL1MessageBitmap []byte
	Chunks                 Chunks
}

func NewCommitBatchDaV0(version uint8, batchIndex uint64, parentBatchHeader *codecv0.DABatch, skippedL1MessageBitmap []byte, chunks Chunks) DAEntry {
	return &CommitBatchDaV0{
		DaType:                 CommitBatchV0,
		Version:                version,
		BatchIndex:             batchIndex,
		ParentBatchHeader:      parentBatchHeader,
		SkippedL1MessageBitmap: skippedL1MessageBitmap,
		Chunks:                 chunks,
	}
}

func (f *CommitBatchDaV0) DAType() DAType {
	return f.DaType
}

type RevertBatchDA struct {
	DaType     DAType
	BatchIndex uint64
}

func NewRevertBatchDA(batchIndex uint64) DAEntry {
	return &FinalizeBatchDA{
		DaType:     RevertBatch,
		BatchIndex: batchIndex,
	}
}

func (f *RevertBatchDA) DAType() DAType {
	return f.DaType
}

type FinalizeBatchDA struct {
	DaType     DAType
	BatchIndex uint64
}

func NewFinalizeBatchDA(batchIndex uint64) DAEntry {
	return &FinalizeBatchDA{
		DaType:     FinalizeBatch,
		BatchIndex: batchIndex,
	}
}

func (f *FinalizeBatchDA) DAType() DAType {
	return f.DaType
}
