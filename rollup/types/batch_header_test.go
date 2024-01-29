package types

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/scroll-tech/go-ethereum/common"
)

func TestNewBatchHeader(t *testing.T) {
	// Without L1 Msg
	templateBlockTrace, err := os.ReadFile("../testdata/blockTrace_02.json")
	assert.NoError(t, err)

	wrappedBlock := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace, wrappedBlock))
	chunk := &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock,
		},
	}
	parentBatchHeader := &BatchHeader{
		version:                1,
		batchIndex:             0,
		l1MessagePopped:        0,
		totalL1MessagePopped:   0,
		dataHash:               common.HexToHash("0x0"),
		parentBatchHash:        common.HexToHash("0x0"),
		skippedL1MessageBitmap: nil,
	}
	batchHeader, err := NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	assert.Equal(t, 0, len(batchHeader.skippedL1MessageBitmap))

	// 1 L1 Msg in 1 bitmap
	templateBlockTrace2, err := os.ReadFile("../testdata/blockTrace_04.json")
	assert.NoError(t, err)

	wrappedBlock2 := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace2, wrappedBlock2))
	chunk = &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock2,
		},
	}
	batchHeader, err = NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	assert.Equal(t, 32, len(batchHeader.skippedL1MessageBitmap))
	expectedBitmap := "00000000000000000000000000000000000000000000000000000000000003ff" // skip first 10
	assert.Equal(t, expectedBitmap, common.Bytes2Hex(batchHeader.skippedL1MessageBitmap))

	// many consecutive L1 Msgs in 1 bitmap, no leading skipped msgs
	templateBlockTrace3, err := os.ReadFile("../testdata/blockTrace_05.json")
	assert.NoError(t, err)

	wrappedBlock3 := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace3, wrappedBlock3))
	chunk = &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock3,
		},
	}
	batchHeader, err = NewBatchHeader(1, 1, 37, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	assert.Equal(t, uint64(5), batchHeader.l1MessagePopped)
	assert.Equal(t, 32, len(batchHeader.skippedL1MessageBitmap))
	expectedBitmap = "0000000000000000000000000000000000000000000000000000000000000000" // all bits are included, so none are skipped
	assert.Equal(t, expectedBitmap, common.Bytes2Hex(batchHeader.skippedL1MessageBitmap))

	// many consecutive L1 Msgs in 1 bitmap, with leading skipped msgs
	chunk = &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock3,
		},
	}
	batchHeader, err = NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	assert.Equal(t, uint64(42), batchHeader.l1MessagePopped)
	assert.Equal(t, 32, len(batchHeader.skippedL1MessageBitmap))
	expectedBitmap = "0000000000000000000000000000000000000000000000000000001fffffffff" // skipped the first 37 messages
	assert.Equal(t, expectedBitmap, common.Bytes2Hex(batchHeader.skippedL1MessageBitmap))

	// many sparse L1 Msgs in 1 bitmap
	templateBlockTrace4, err := os.ReadFile("../testdata/blockTrace_06.json")
	assert.NoError(t, err)

	wrappedBlock4 := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace4, wrappedBlock4))
	chunk = &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock4,
		},
	}
	batchHeader, err = NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	assert.Equal(t, uint64(10), batchHeader.l1MessagePopped)
	assert.Equal(t, 32, len(batchHeader.skippedL1MessageBitmap))
	expectedBitmap = "00000000000000000000000000000000000000000000000000000000000001dd" // 0111011101
	assert.Equal(t, expectedBitmap, common.Bytes2Hex(batchHeader.skippedL1MessageBitmap))

	// many L1 Msgs in each of 2 bitmaps
	templateBlockTrace5, err := os.ReadFile("../testdata/blockTrace_07.json")
	assert.NoError(t, err)

	wrappedBlock5 := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace5, wrappedBlock5))
	chunk = &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock5,
		},
	}
	batchHeader, err = NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	assert.Equal(t, uint64(257), batchHeader.l1MessagePopped)
	assert.Equal(t, 64, len(batchHeader.skippedL1MessageBitmap))
	expectedBitmap = "fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffd0000000000000000000000000000000000000000000000000000000000000000"
	assert.Equal(t, expectedBitmap, common.Bytes2Hex(batchHeader.skippedL1MessageBitmap))
}

func TestBatchHeaderEncode(t *testing.T) {
	// Without L1 Msg
	templateBlockTrace, err := os.ReadFile("../testdata/blockTrace_02.json")
	assert.NoError(t, err)

	wrappedBlock := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace, wrappedBlock))
	chunk := &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock,
		},
	}
	parentBatchHeader := &BatchHeader{
		version:                1,
		batchIndex:             0,
		l1MessagePopped:        0,
		totalL1MessagePopped:   0,
		dataHash:               common.HexToHash("0x0"),
		parentBatchHash:        common.HexToHash("0x0"),
		skippedL1MessageBitmap: nil,
	}
	batchHeader, err := NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	bytes := batchHeader.Encode()
	assert.Equal(t, 89, len(bytes))
	assert.Equal(t, "010000000000000001000000000000000000000000000000008fbc5eecfefc5bd9d1618ecef1fed160a7838448383595a2257d4c9bd5c5fa3e4136709aabc8a23aa17fbcc833da2f7857d3c2884feec9aae73429c135f94985", common.Bytes2Hex(bytes))

	// With L1 Msg
	templateBlockTrace2, err := os.ReadFile("../testdata/blockTrace_04.json")
	assert.NoError(t, err)

	wrappedBlock2 := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace2, wrappedBlock2))
	chunk = &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock2,
		},
	}
	batchHeader, err = NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	bytes = batchHeader.Encode()
	assert.Equal(t, 121, len(bytes))
	assert.Equal(t, "010000000000000001000000000000000b000000000000000be97c01f25506dfc76f776bcefa0f0d880ef734b2bf7d63d5d1fc9d3be13006324136709aabc8a23aa17fbcc833da2f7857d3c2884feec9aae73429c135f9498500000000000000000000000000000000000000000000000000000000000003ff", common.Bytes2Hex(bytes))
}

func TestBatchHeaderHash(t *testing.T) {
	// Without L1 Msg
	templateBlockTrace, err := os.ReadFile("../testdata/blockTrace_02.json")
	assert.NoError(t, err)

	wrappedBlock := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace, wrappedBlock))
	chunk := &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock,
		},
	}
	parentBatchHeader := &BatchHeader{
		version:                1,
		batchIndex:             0,
		l1MessagePopped:        0,
		totalL1MessagePopped:   0,
		dataHash:               common.HexToHash("0x0"),
		parentBatchHash:        common.HexToHash("0x0"),
		skippedL1MessageBitmap: nil,
	}
	batchHeader, err := NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	hash := batchHeader.Hash()
	assert.Equal(t, "b0bd69b1c27556049569d09be617c4d0ed656f737b0de9e582d723be264420c6", common.Bytes2Hex(hash.Bytes()))

	templateBlockTrace, err = os.ReadFile("../testdata/blockTrace_03.json")
	assert.NoError(t, err)

	wrappedBlock2 := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace, wrappedBlock2))
	chunk2 := &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock2,
		},
	}
	batchHeader2, err := NewBatchHeader(1, 2, 0, batchHeader.Hash(), []*Chunk{chunk2})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader2)
	hash2 := batchHeader2.Hash()
	assert.Equal(t, "0d44f247c6542892ba4a0f57d3b69b5d0af5c7510b6dc1bf4664e694fbe402b6", common.Bytes2Hex(hash2.Bytes()))

	// With L1 Msg
	templateBlockTrace3, err := os.ReadFile("../testdata/blockTrace_04.json")
	assert.NoError(t, err)

	wrappedBlock3 := &WrappedBlock{}
	assert.NoError(t, json.Unmarshal(templateBlockTrace3, wrappedBlock3))
	chunk = &Chunk{
		Blocks: []*WrappedBlock{
			wrappedBlock3,
		},
	}
	batchHeader, err = NewBatchHeader(1, 1, 0, parentBatchHeader.Hash(), []*Chunk{chunk})
	assert.NoError(t, err)
	assert.NotNil(t, batchHeader)
	hash = batchHeader.Hash()
	assert.Equal(t, "1922fc751323830f04d6d9ea6330ecb484e09a4a4f12dc7d53f1fdb0f56c8f53", common.Bytes2Hex(hash.Bytes()))
}

func TestBatchHeaderDecode(t *testing.T) {
	header := &BatchHeader{
		version:                1,
		batchIndex:             10,
		l1MessagePopped:        20,
		totalL1MessagePopped:   30,
		dataHash:               common.HexToHash("0x01"),
		parentBatchHash:        common.HexToHash("0x02"),
		skippedL1MessageBitmap: []byte{0x01, 0x02, 0x03},
	}

	encoded := header.Encode()
	decoded, err := DecodeBatchHeader(encoded)
	assert.NoError(t, err)
	assert.Equal(t, header, decoded)
}
