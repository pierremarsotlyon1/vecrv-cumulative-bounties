package utils

import (
	"main/contracts/erc20"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var tokens = make(map[common.Address]uint8, 0)

func GetTokenDecimals(client *ethclient.Client, token common.Address) (uint8, error) {
	decimals, exists := tokens[token]
	if exists {
		return decimals, nil
	}

	tokenContract, err := erc20.NewErc20(token, client)
	if err != nil {
		return 0, err
	}

	tokenDecimals, err := tokenContract.Decimals(nil)
	if err != nil {
		return 0, err
	}

	tokens[token] = tokenDecimals

	return tokenDecimals, nil
}
