package utils

import (
	"encoding/json"
	"io"
	"main/interfaces"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var tokenPrices = make(map[string]float64, 0)
var priceMutex sync.Mutex

func GetHistoricalPriceTokenPrice(token common.Address, chainName string, timestamp uint64) float64 {
	priceMutex.Lock()
	defer priceMutex.Unlock()

	url := "https://coins.llama.fi/prices/historical/" + strconv.FormatUint(timestamp, 10) + "/" + chainName + ":" + token.Hex()

	price, exists := tokenPrices[url]
	if exists {
		return price
	}

	response, err := http.Get(url)

	// For limit rate
	time.Sleep(500 * time.Millisecond)

	if err != nil {
		tokenPrices[url] = 0
		return 0
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		tokenPrices[url] = 0
		return 0
	}

	defilammaPrice := new(interfaces.DefilammaPrice)
	if err := json.Unmarshal(body, defilammaPrice); err != nil {
		tokenPrices[url] = 0
		return 0
	}

	obj, exists := defilammaPrice.Coins[chainName+":"+token.Hex()]
	if !exists {
		tokenPrices[url] = 0
		return 0
	}

	tokenPrices[url] = obj.Price

	return obj.Price
}
