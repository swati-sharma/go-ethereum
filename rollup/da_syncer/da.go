package da_syncer

type DAType int

const (
	// CommitBatch contains data of event of CommitBatch
	CommitBatch DAType = iota
	// RevertBatch contains data of event of RevertBatch
	RevertBatch
	// FinalizeBatch contains data of event of FinalizeBatch
	FinalizeBatch
)

type DAEntry struct {
	// DaType is a type of DA entry (CommitBatch, RevertBatch, FinalizeBatch)
	DaType DAType
	// BatchIndex index of batch
	BatchIndex uint64
	// Chunks contains chunk of a batch
	Chunks Chunks
	// L1Txs contains l1txs of a batch
	L1Txs L1Txs
}

type DA []*DAEntry

func NewCommitBatchDA(batchIndex uint64, chunks Chunks, l1txs L1Txs) *DAEntry {
	return &DAEntry{
		DaType:     CommitBatch,
		BatchIndex: batchIndex,
		Chunks:     chunks,
		L1Txs:      l1txs,
	}
}

func NewRevertBatchDA(batchIndex uint64) *DAEntry {
	return &DAEntry{
		DaType:     RevertBatch,
		BatchIndex: batchIndex,
	}
}

func NewFinalizeBatchDA(batchIndex uint64) *DAEntry {
	return &DAEntry{
		DaType:     FinalizeBatch,
		BatchIndex: batchIndex,
	}
}
