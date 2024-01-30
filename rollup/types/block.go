package types

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"

	"github.com/scroll-tech/go-ethereum/common"
	"github.com/scroll-tech/go-ethereum/core/types"
	"github.com/scroll-tech/go-ethereum/log"
)

const CalldataNonZeroByteGas = 16
const blockContextByteSize = 60

// GetKeccak256Gas calculates the gas cost for computing the keccak256 hash of a given size.
func GetKeccak256Gas(size uint64) uint64 {
	return GetMemoryExpansionCost(size) + 30 + 6*((size+31)/32)
}

// GetMemoryExpansionCost calculates the cost of memory expansion for a given memoryByteSize.
func GetMemoryExpansionCost(memoryByteSize uint64) uint64 {
	memorySizeWord := (memoryByteSize + 31) / 32
	memoryCost := (memorySizeWord*memorySizeWord)/512 + (3 * memorySizeWord)
	return memoryCost
}

// WrappedBlock contains the block's Header, Transactions and WithdrawTrieRoot hash.
type WrappedBlock struct {
	Header               *types.Header         `json:"header"`
	Transactions         []*types.Transaction  `json:"transactions"`
	WithdrawRoot         common.Hash           `json:"withdraw_trie_root"`
	RowConsumption       *types.RowConsumption `json:"row_consumption"`
	txPayloadLengthCache map[common.Hash]uint64
}

// BlockContext represents the essential data of a block in the ScrollChain.
// It provides an overview of block attributes including hash values, block numbers, gas details, and transaction counts.
type BlockContext struct {
	BlockHash       common.Hash
	ParentHash      common.Hash
	BlockNumber     uint64
	Timestamp       uint64
	BaseFee         *big.Int
	GasLimit        uint64
	NumTransactions uint16
	NumL1Messages   uint16
}

// Encode encodes the WrappedBlock into RollupV2 BlockContext Encoding.
func (w *WrappedBlock) Encode(totalL1MessagePoppedBefore uint64) ([]byte, error) {
	bytes := make([]byte, blockContextByteSize)

	if !w.Header.Number.IsUint64() {
		return nil, errors.New("block number is not uint64")
	}

	numL1Messages := w.NumL1Messages(totalL1MessagePoppedBefore)
	if numL1Messages > math.MaxUint16 {
		return nil, errors.New("number of L1 messages exceeds max uint16")
	}

	numL2Transactions := w.NumL2Transactions()
	numTransactions := numL1Messages + numL2Transactions
	if numTransactions > math.MaxUint16 {
		return nil, errors.New("number of transactions exceeds max uint16")
	}

	binary.BigEndian.PutUint64(bytes[0:], w.Header.Number.Uint64())
	binary.BigEndian.PutUint64(bytes[8:], w.Header.Time)
	if w.Header.BaseFee != nil {
		binary.BigEndian.PutUint64(bytes[40:], w.Header.BaseFee.Uint64())
	}
	binary.BigEndian.PutUint64(bytes[48:], w.Header.GasLimit)
	binary.BigEndian.PutUint16(bytes[56:], uint16(numTransactions))
	binary.BigEndian.PutUint16(bytes[58:], uint16(numL1Messages))

	return bytes, nil
}

// NumL1Messages returns the number of L1 messages in this block.
// This number is the sum of included and skipped L1 messages.
func (w *WrappedBlock) NumL1Messages(totalL1MessagePoppedBefore uint64) uint64 {
	var lastQueueIndex *uint64
	for _, tx := range w.Transactions {
		if tx.Type() == types.L1MessageTxType {
			lastQueueIndex = &tx.AsL1MessageTx().QueueIndex
		}
	}
	if lastQueueIndex == nil {
		return 0
	}
	return *lastQueueIndex - totalL1MessagePoppedBefore + 1
}

// NumL2Transactions returns the number of L2 transactions in this block.
func (w *WrappedBlock) NumL2Transactions() uint64 {
	var count uint64
	for _, tx := range w.Transactions {
		if tx.Type() != types.L1MessageTxType {
			count++
		}
	}
	return count
}

// EstimateL1CommitCalldataSize calculates the calldata size in l1 commit approximately.
// TODO: The calculation could be more accurate by using 58 + len(l2TxDataBytes) (see Chunk).
// This needs to be adjusted in the future.
func (w *WrappedBlock) EstimateL1CommitCalldataSize() uint64 {
	var size uint64
	for _, tx := range w.Transactions {
		if tx.Type() == types.L1MessageTxType {
			continue
		}
		size += 4 // 4 bytes payload length
		size += w.getTxPayloadLength(tx)
	}
	size += blockContextByteSize //  60 bytes BlockContext
	return size
}

// EstimateL1CommitGas calculates the total L1 commit gas for this block approximately.
func (w *WrappedBlock) EstimateL1CommitGas() uint64 {
	var total uint64
	var numL1Messages uint64
	for _, tx := range w.Transactions {
		if tx.Type() == types.L1MessageTxType {
			numL1Messages++
			continue
		}

		txPayloadLength := w.getTxPayloadLength(tx)
		total += CalldataNonZeroByteGas * txPayloadLength // an over-estimate: treat each byte as non-zero
		total += CalldataNonZeroByteGas * 4               // 4 bytes payload length
		total += GetKeccak256Gas(txPayloadLength)         // l2 tx hash
	}

	// 60 bytes BlockContext calldata
	total += CalldataNonZeroByteGas * blockContextByteSize

	// sload
	total += 2100 * numL1Messages // numL1Messages times cold sload in L1MessageQueue

	// staticcall
	total += 100 * numL1Messages // numL1Messages times call to L1MessageQueue
	total += 100 * numL1Messages // numL1Messages times warm address access to L1MessageQueue

	total += GetMemoryExpansionCost(36) * numL1Messages // staticcall to proxy
	total += 100 * numL1Messages                        // read admin in proxy
	total += 100 * numL1Messages                        // read impl in proxy
	total += 100 * numL1Messages                        // access impl
	total += GetMemoryExpansionCost(36) * numL1Messages // delegatecall to impl

	return total
}

func (w *WrappedBlock) getTxPayloadLength(tx *types.Transaction) uint64 {
	if w.txPayloadLengthCache == nil {
		w.txPayloadLengthCache = make(map[common.Hash]uint64)
	}

	if length, exists := w.txPayloadLengthCache[tx.Hash()]; exists {
		return length
	}

	rlp, err := tx.MarshalBinary()
	if err != nil {
		log.Crit("marshal binary failed, which should not happen", "hash", tx.Hash().String(), "err", err)
		return 0
	}
	txPayloadLength := uint64(len(rlp))
	w.txPayloadLengthCache[tx.Hash()] = txPayloadLength
	return txPayloadLength
}

func decodeBlockContext(encodedBlockContext []byte) (*BlockContext, error) {
	if len(encodedBlockContext) != blockContextByteSize {
		return nil, errors.New("block encoding is not 60 bytes long")
	}

	return &BlockContext{
		BlockNumber:     binary.BigEndian.Uint64(encodedBlockContext[0:8]),
		Timestamp:       binary.BigEndian.Uint64(encodedBlockContext[8:16]),
		BaseFee:         new(big.Int).SetUint64(binary.BigEndian.Uint64(encodedBlockContext[40:48])),
		GasLimit:        binary.BigEndian.Uint64(encodedBlockContext[48:56]),
		NumTransactions: binary.BigEndian.Uint16(encodedBlockContext[56:58]),
		NumL1Messages:   binary.BigEndian.Uint16(encodedBlockContext[58:60]),
	}, nil
}
