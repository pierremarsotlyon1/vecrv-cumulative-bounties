package interfaces

import "math/big"

type Lock struct {
	Timestamp uint64   `json:"timestamp"`
	Value     *big.Int `json:"value"`
}
