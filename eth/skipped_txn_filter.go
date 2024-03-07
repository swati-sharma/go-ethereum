package eth

import (
	"context"

	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/eth/tracers"
	"github.com/scroll-tech/go-ethereum/rollup/circuitcapacitychecker"
	"github.com/scroll-tech/go-ethereum/rpc"
)

type NewCircuitCapacityChecker func() *circuitcapacitychecker.CircuitCapacityChecker

type CurrentHeaderReader interface {
	CurrentHeader() *types.Header
}

type SkippedTxnFilter struct {
	headReader                CurrentHeaderReader
	tracer                    tracers.TraceBlock
	newCircuitCapacityChecker NewCircuitCapacityChecker
}

func NewSkippedTxnFilter(headReader CurrentHeaderReader, tracer tracers.TraceBlock, newCircuitCapacityChecker NewCircuitCapacityChecker) *SkippedTxnFilter {
	return &SkippedTxnFilter{
		headReader:                headReader,
		tracer:                    tracer,
		newCircuitCapacityChecker: newCircuitCapacityChecker,
	}
}

func (f *SkippedTxnFilter) Apply(txn *types.Transaction, local bool) error {
	// always execute on the latest block
	blockToExecuteOn := rpc.BlockNumberOrHashWithHash(f.headReader.CurrentHeader().Hash(), false)
	trace, err := f.tracer.GetTxBlockTraceOnTopOfBlock(context.Background(), txn, blockToExecuteOn, nil)
	if err != nil {
		return err
	}
	_, err = f.newCircuitCapacityChecker().ApplyTransaction(trace)
	return err
}
