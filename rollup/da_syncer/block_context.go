package da_syncer

import (
	"encoding/binary"
	"errors"
	"math/big"
)

// BlockContext represents the essential data of a block in the ScrollChain.
// It provides an overview of block attributes including hash values, block numbers, gas details, and transaction counts.
type BlockContext struct {
	BlockNumber     uint64
	Timestamp       uint64
	BaseFee         *big.Int
	GasLimit        uint64
	NumTransactions uint16
	NumL1Messages   uint16
}

type BlockContexts []*BlockContext

func decodeBlockContext(encodedBlockContext []byte) (*BlockContext, error) {
	if len(encodedBlockContext) != blockContextByteSize {
		return nil, errors.New("block encoding is not 60 bytes long")
	}
	baseFee := big.NewInt(0).SetBytes(encodedBlockContext[16:48])

	return &BlockContext{
		BlockNumber:     binary.BigEndian.Uint64(encodedBlockContext[0:8]),
		Timestamp:       binary.BigEndian.Uint64(encodedBlockContext[8:16]),
		BaseFee:         baseFee,
		GasLimit:        binary.BigEndian.Uint64(encodedBlockContext[48:56]),
		NumTransactions: binary.BigEndian.Uint16(encodedBlockContext[56:58]),
		NumL1Messages:   binary.BigEndian.Uint16(encodedBlockContext[58:60]),
	}, nil
}
