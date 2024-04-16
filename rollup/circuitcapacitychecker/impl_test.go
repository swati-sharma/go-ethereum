package circuitcapacitychecker

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/scroll-tech/go-ethereum/core/types"
)

func BenchmarkApplyTransaction(b *testing.B) {
	data, err := os.ReadFile("block_trace_0x084d4643046fa33e2b832d74b5fd74d4cdf04a5eb93d048e3764b5883a29060f.json")
	if err != nil {
		b.Fatalf("Error reading block trace file: %v", err)
	}

	var blockTrace types.BlockTrace
	err = json.Unmarshal(data, &blockTrace)
	if err != nil {
		b.Fatalf("Error unmarshaling block trace JSON: %v", err)
	}

	ccc := NewCircuitCapacityChecker(true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ccc.ApplyTransaction(&blockTrace)
		if err != nil {
			b.Fatalf("Error applying transaction: %v", err)
		}
	}
}
