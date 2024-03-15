package utils

import (
	"main/contracts/erc20"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var tokens = make(map[common.Address]uint8, 0)
var tokenMutex sync.Mutex

func GetTokenDecimals(client *ethclient.Client, token common.Address) (uint8, error) {
	if strings.EqualFold(ETH_DEFAULT.Hex(), token.Hex()) {
		return 18, nil
	}

	tokenMutex.Lock()
	defer tokenMutex.Unlock()

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

func GetDefilammaChain(chainName string) string {
	switch chainName {
	case "polygon":
		return "polygon"
	case "arbitrum":
		return "arbitrum"
	case "base":
		return "base"
	case "celo":
		return "celo"
	case "optimism":
		return "optimism"
	case "xdai":
		return "gnosis"
	case "avalanche":
		return "avax"
	case "fantom":
		return "coingecko"
	case "kava":
		return "kava"
	case "moonbeam":
		return "moonbeam"
	default:
		return "ethereum"
	}
}
