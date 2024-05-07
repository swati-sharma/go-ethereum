package stateprocessor

import (
	"fmt"
	"math"
	"time"

	"github.com/scroll-tech/go-ethereum/core"
	"github.com/scroll-tech/go-ethereum/core/state"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/core/vm"
	"github.com/scroll-tech/go-ethereum/log"
	"github.com/scroll-tech/go-ethereum/rollup/circuitcapacitychecker"
	"github.com/scroll-tech/go-ethereum/rollup/pipeline"
)

var _ core.Processor = (*Processor)(nil)
var max float64
var min = math.Inf(0)
var speedups []float64

type Processor struct {
	chain *core.BlockChain
	ccc   *circuitcapacitychecker.CircuitCapacityChecker
}

func NewProcessor(bc *core.BlockChain) *Processor {
	return &Processor{
		chain: bc,
		ccc:   circuitcapacitychecker.NewCircuitCapacityChecker(true),
	}
}

func (p *Processor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	if block.Transactions().Len() == 0 {
		return types.Receipts{}, []*types.Log{}, 0, nil
	}

	header := block.Header()
	header.GasUsed = 0

	pl := pipeline.NewPipeline(p.chain, &cfg, statedb, nil, header, p.ccc)
	pl.Start(time.Now().Add(time.Minute))
	defer pl.Kill()

	for _, tx := range block.Transactions() {
		res, err := pl.TryPushTxn(tx)
		if err != nil {
			return nil, nil, 0, err
		}

		if res != nil {
			return nil, nil, 0, fmt.Errorf("pipeline ended prematurely %v", res.CCCErr)
		}
	}
	close(pl.TxnQueue)
	pl.TxnQueue = nil
	res := <-pl.ResultCh
	if res.CCCErr != nil {
		return nil, nil, 0, res.CCCErr
	}
	speedup := float64((pl.ApplyTimer + pl.CccTimer).Microseconds()) / float64(pl.LifetimeTimer.Microseconds())
	if speedup < min {
		min = speedup
	}
	if speedup > max {
		max = speedup
	}

	speedups = append(speedups, speedup)
	if len(speedups) > 500 {
		speedups = speedups[1:]
	}
	sum := float64(0)
	for i := 0; i < len(speedups); i++ {
		sum += (speedups[i])
	}
	avgspeedup := (float64(sum)) / (float64(len(speedups)))

	log.Info("PStats", "lifetime", pl.LifetimeTimer, "apply", pl.ApplyTimer, "apply_idle", pl.ApplyIdleTimer,
		"apply_stall", pl.ApplyStallTimer, "ccc", pl.CccTimer, "ccc_idle", pl.CccIdleTimer, "speedup", speedup, "min", min, "max", max, "avg", avgspeedup)
	return res.FinalBlock.Receipts, res.FinalBlock.CoalescedLogs, res.FinalBlock.Header.GasUsed, nil
}
