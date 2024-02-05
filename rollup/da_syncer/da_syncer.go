package da_syncer

import (
	"context"
	"time"

	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/log"
)

// defaultSyncInterval is the frequency at which we query for new rollup event.
const defaultSyncInterval = 60 * time.Second

type DaSyncer struct {
	DaFetcher DaFetcher
	ctx       context.Context
	cancel    context.CancelFunc
	// batches is map from batchIndex to batch blocks
	batches map[uint64][]*types.Block
}

func NewDaSyncer(ctx context.Context, daFetcher DaFetcher) (*DaSyncer, error) {
	ctx, cancel := context.WithCancel(ctx)
	daSyncer := DaSyncer{
		DaFetcher: daFetcher,
		ctx:       ctx,
		cancel:    cancel,
		batches:   make(map[uint64][]*types.Block),
	}
	return &daSyncer, nil
}

func (s *DaSyncer) Start() {
	if s == nil {
		return
	}

	log.Info("Starting DaSyncer")

	go func() {
		syncTicker := time.NewTicker(defaultSyncInterval)
		defer syncTicker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-syncTicker.C:
				s.syncWithDa()
			}
		}
	}()
}

func (s *DaSyncer) Stop() {
	if s == nil {
		return
	}

	log.Info("Stopping DaSyncer")

	if s.cancel != nil {
		s.cancel()
	}
}

func (s *DaSyncer) syncWithDa() {
	da, err := s.DaFetcher.FetchDA()
	if err != nil {
		log.Error("failed to fetch DA", "err", err)
		return
	}
	for _, daEntry := range da {
		switch daEntry.DaType {
		case CommitBatch:
			blocks, err := s.processDaToBlocks(daEntry)
			if err != nil {
				log.Warn("failed to process DA to blocks", "err", err)
				return
			}
			s.batches[daEntry.BatchIndex] = blocks
		case RevertBatch:
			delete(s.batches, daEntry.BatchIndex)
		case FinalizeBatch:
			blocks, ok := s.batches[daEntry.BatchIndex]
			if !ok {
				log.Warn("cannot find blocks for batch", "batch index", daEntry.BatchIndex, "err", err)
				return
			}
			s.insertBlocks(blocks)
		}
	}
}

func (s *DaSyncer) processDaToBlocks(daEntry *DAEntry) ([]*types.Block, error) {
	return nil, nil
}

func (s *DaSyncer) insertBlocks([]*types.Block) {

}
