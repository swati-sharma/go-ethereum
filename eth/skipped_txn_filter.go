package eth

import (
	"context"

	"github.com/scroll-tech/go-ethereum/common/hexutil"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/eth/tracers"
	"github.com/scroll-tech/go-ethereum/internal/ethapi"
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
	signer                    types.Signer
}

func NewSkippedTxnFilter(headReader CurrentHeaderReader, tracer tracers.TraceBlock, newCircuitCapacityChecker NewCircuitCapacityChecker, signer types.Signer) *SkippedTxnFilter {
	return &SkippedTxnFilter{
		headReader:                headReader,
		tracer:                    tracer,
		newCircuitCapacityChecker: newCircuitCapacityChecker,
		signer:                    signer,
	}
}

func (f *SkippedTxnFilter) Apply(txn *types.Transaction, local bool) error {
	sender, err := f.signer.Sender(txn)
	if err != nil {
		return err
	}
	nonce := txn.Nonce()

	// always execute on the latest block
	blockToExecuteOn := rpc.BlockNumberOrHashWithHash(f.headReader.CurrentHeader().Hash(), false)
	// override nonce to account for pending txns in the pool
	trace, err := f.tracer.GetTxBlockTraceOnTopOfBlock(context.Background(), txn, blockToExecuteOn, &tracers.TraceConfig{
		StateOverrides: &ethapi.StateOverride{
			sender: ethapi.OverrideAccount{
				Nonce: (*hexutil.Uint64)(&nonce),
			},
		},
	})
	if err != nil {
		return err
	}
	_, err = f.newCircuitCapacityChecker().ApplyTransaction(trace)
	return err
}
