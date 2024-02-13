package da_syncer

import (
	"encoding/binary"
	"fmt"
)

const blockContextByteSize = 60

type Chunk struct {
	BlockContexts BlockContexts
	L2Txs         [][]byte
}

type Chunks []*Chunk

// decodeChunks decodes the provided chunks into a list of chunks.
func decodeChunks(chunksData [][]byte) (Chunks, error) {
	var chunks Chunks
	for _, chunk := range chunksData {
		if len(chunk) < 1 {
			return nil, fmt.Errorf("invalid chunk, length is less than 1")
		}

		numBlocks := int(chunk[0])
		if len(chunk) < 1+numBlocks*blockContextByteSize {
			return nil, fmt.Errorf("chunk size doesn't match with numBlocks, byte length of chunk: %v, expected length: %v", len(chunk), 1+numBlocks*blockContextByteSize)
		}

		blockContexts := make(BlockContexts, numBlocks)
		for i := 0; i < numBlocks; i++ {
			startIdx := 1 + i*blockContextByteSize // add 1 to skip numBlocks byte
			endIdx := startIdx + blockContextByteSize
			blockContext, err := decodeBlockContext(chunk[startIdx:endIdx])
			if err != nil {
				return nil, err
			}
			blockContexts[i] = blockContext
		}

		var l2Txs [][]byte
		txLen := 0

		for currentIndex := 1 + numBlocks*blockContextByteSize; currentIndex < len(chunk); currentIndex += 4 + txLen {
			if len(chunk) < currentIndex+4 {
				return nil, fmt.Errorf("chunk size doesn't match, next tx size is less then 4, byte length of chunk: %v, expected length: %v", len(chunk), currentIndex+4)
			}
			txLen = int(binary.BigEndian.Uint32(chunk[currentIndex : currentIndex+4]))
			if len(chunk) < currentIndex+4+txLen {
				return nil, fmt.Errorf("chunk size doesn't match with next tx length, byte length of chunk: %v, expected length: %v", len(chunk), currentIndex+4+txLen)
			}
			txData := chunk[currentIndex+4 : currentIndex+4+txLen]
			l2Txs = append(l2Txs, txData)
		}

		chunks = append(chunks, &Chunk{
			BlockContexts: blockContexts,
			L2Txs:         l2Txs,
		})
	}
	return chunks, nil
}

func countTotalL1MessagePopped(chunks Chunks) uint64 {
	var total uint64 = 0
	for _, chunk := range chunks {
		for _, block := range chunk.BlockContexts {
			total += uint64(block.NumL1Messages)
		}
	}
	return total
}
