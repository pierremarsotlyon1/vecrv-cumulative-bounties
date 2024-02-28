package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"main/contracts/erc20"
	"main/contracts/questDistributor"
	"main/contracts/votemarketV1"
	"main/contracts/votemarketV2"
	"main/contracts/votiumMerkle"
	"main/contracts/yBribeV3"
	"main/interfaces"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/ethclient"
)

var VOTIUM_ADDRESSES = []common.Address{
	//common.HexToAddress("0x378Ba9B73309bE80BF4C2c027aAD799766a7ED5A"), => seems vlCVX
	common.HexToAddress("0x34590960981f98b55d236b70E8B4d9929ad89C9c"),
}

var VOTEMARKET_ADDRESSES_V1 = []common.Address{
	common.HexToAddress("0x7D0F747eb583D43D41897994c983F13eF7459e1f"),
}

var VOTEMARKET_ADDRESSES_V2 = []common.Address{
	common.HexToAddress("0x0000000BE1d98523B5469AfF51A1e7b4891c6225"),
	common.HexToAddress("0x0000000895cB182E6f983eb4D8b4E0Aa0B31Ae4c"),
}

var QUEST_ADDRESSES = []common.Address{
	common.HexToAddress("0x3682518b529e4404fb05250F9ad590C3218E5F9f"),
	common.HexToAddress("0xce6dc32252d85e2e955Bfd3b85660917F040a933"),
}

var YBRIBE_ADDRESSES = []common.Address{
	common.HexToAddress("0x03dFdBcD4056E2F92251c7B07423E1a33a7D3F6d"),
}

var tokens = make(map[common.Address]uint8, 0)
var tokenPrices = make(map[string]float64, 0)

const RPC_URL = "/datastore/.ethereum/geth.ipc"
const ALCHEMY_RPC_URL = ""
const DATA_PATH = "./data.json"

func main() {

	if len(RPC_URL) == 0 {
		fmt.Println("Main RPC url not set")
	}

	if len(ALCHEMY_RPC_URL) == 0 {
		fmt.Println("Alchemy RPC url not set")
		return
	}

	client, err := ethclient.Dial(RPC_URL)
	if err != nil {
		panic(err)
	}

	allClaimed := make([]interfaces.BountyClaimed, 0)

	if _, err := os.Stat(DATA_PATH); err != nil {

		fmt.Println("Fetching votium")
		allClaimed = append(allClaimed, fetchVotium(client)...)

		fmt.Println("Fetching votemarket")
		allClaimed = append(allClaimed, fetchVotemarketV1(client)...)
		allClaimed = append(allClaimed, fetchVotemarketV2(client)...)

		fmt.Println("Fetching quest")
		allClaimed = append(allClaimed, fetchQuest(client)...)
		fmt.Println("Fetching yBribe")
		allClaimed = append(allClaimed, fetchYBribe(client)...)

		file, err := json.Marshal(allClaimed)
		if err != nil {
			log.Fatal(err)
		}

		if _, err := os.Create(DATA_PATH); err != nil {
			log.Fatal(err)
		}

		if err := os.WriteFile(DATA_PATH, file, 0644); err != nil {
			log.Fatal(err)
		}
	} else {
		file, err := os.ReadFile(DATA_PATH)
		if err != nil {
			log.Fatal(err)
		}

		if err := json.Unmarshal([]byte(file), &allClaimed); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println(len(allClaimed), " claims found")

	totalBountiesUSD := 0.0
	for _, bounty := range allClaimed {

		price, exists := tokenPrices[bounty.TokenReward.Hex()]
		if !exists {
			tokenPrice := getTokenPrice(bounty.TokenReward)
			if tokenPrice == 0 {
				tokenPrice = getTokenPriceFromGeckoterminal(bounty.TokenReward)
			}

			tokenPrices[bounty.TokenReward.Hex()] = tokenPrice
			price = tokenPrice

			if tokenPrice == 0 {
				fmt.Println("Price == 0 for ", bounty.TokenReward.Hex())
				tokenPrices[bounty.TokenReward.Hex()] = 0
				continue
			}
		}

		bounty.CurrentPrice = price

		amount, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(bounty.Amount), big.NewFloat(0).SetInt(math.BigPow(10, int64(bounty.TokenDecimals)))).Float64()
		totalBountiesUSD += (amount * price)
	}

	file, err := json.Marshal(allClaimed)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(DATA_PATH, file, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Total bounties USD : ", fmt.Sprintf("%f", totalBountiesUSD))
}

func fetchVotium(client *ethclient.Client) []interfaces.BountyClaimed {

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(13320169),
		ToBlock:   nil, // Latest
		Addresses: VOTIUM_ADDRESSES,
		Topics:    [][]common.Hash{{common.HexToHash("0x4766921f5c59646d22d7d266a29164c8e9623684d8dfdbd931731dfdca025238")}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)

	for _, vLog := range logs {
		votiumContract, err := votiumMerkle.NewVotiumMerkle(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		event, err := votiumContract.ParseClaimed(vLog)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.Token, vLog.BlockNumber, event.Amount, vLog.TxHash)
	}

	return bountiesClaimed
}

func fetchVotemarketV1(client *ethclient.Client) []interfaces.BountyClaimed {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(16376672),
		ToBlock:   nil, // Latest
		Addresses: VOTEMARKET_ADDRESSES_V1,
		Topics:    [][]common.Hash{{common.HexToHash("0x6f9c9826be5976f3f82a3490c52a83328ce2ec7be9e62dcb39c26da5148d7c76")}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)

	for _, vLog := range logs {
		votemarketContract, err := votemarketV1.NewVotemarketV1(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		event, err := votemarketContract.ParseClaimed(vLog)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash)
	}

	return bountiesClaimed
}

func fetchVotemarketV2(client *ethclient.Client) []interfaces.BountyClaimed {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(16376672),
		ToBlock:   nil, // Latest
		Addresses: VOTEMARKET_ADDRESSES_V2,
		Topics:    [][]common.Hash{{common.HexToHash("0x6f9c9826be5976f3f82a3490c52a83328ce2ec7be9e62dcb39c26da5148d7c76")}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)

	for _, vLog := range logs {
		votemarketContract, err := votemarketV2.NewVotemarketV2(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		event, err := votemarketContract.ParseClaimed(vLog)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash)
	}

	return bountiesClaimed
}

func fetchQuest(client *ethclient.Client) []interfaces.BountyClaimed {

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(14784921),
		ToBlock:   nil, // Latest
		Addresses: QUEST_ADDRESSES,
		Topics:    [][]common.Hash{{common.HexToHash("0x9a5376f7dcf8631c2b6249c9bec3d715cb97bdd4c82d92e55d147f6b4eea4197")}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)

	for _, vLog := range logs {
		questContract, err := questDistributor.NewQuestDistributor(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		event, err := questContract.ParseClaimed(vLog)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash)
	}

	return bountiesClaimed

}

func fetchYBribe(client *ethclient.Client) []interfaces.BountyClaimed {
	client2, err := ethclient.Dial(ALCHEMY_RPC_URL)
	if err != nil {
		panic(err)
	}

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(15878262),
		ToBlock:   nil, // Latest
		Addresses: YBRIBE_ADDRESSES,
		Topics:    [][]common.Hash{{common.HexToHash("0x2422cac5e23c46c890fdcf42d0c64757409df6832174df639337558f09d99c68")}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)

	for _, vLog := range logs {
		ybribeContract, err := yBribeV3.NewYBribeV3(vLog.Address, client)
		if err != nil {
			fmt.Println(err)
			continue
		}

		event, err := ybribeContract.ParseRewardClaimed(vLog)
		if err != nil {
			panic(err)
		}

		receipt, err := client2.TransactionReceipt(context.Background(), vLog.TxHash)
		if err != nil {
			fmt.Println(err)
			continue
		}

		found := false
		for _, receiptLog := range receipt.Logs {
			if strings.EqualFold(receiptLog.Topics[0].Hex(), common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef").Hex()) {
				t, err := erc20.NewErc20(receiptLog.Address, client)
				if err != nil {
					continue
				}

				transfert, err := t.ParseTransfer(*receiptLog)
				if err != nil {
					continue
				}

				if transfert.Value.Cmp(event.Amount) != 0 {
					continue
				}

				event.RewardToken = receiptLog.Address
				found = true
				break
			}
		}

		if !found {
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash)
	}

	return bountiesClaimed
}

func addClaim(client *ethclient.Client, bountiesClaimed []interfaces.BountyClaimed, contract common.Address, rewardToken common.Address, blockNumber uint64, amount *big.Int, txHash common.Hash) []interfaces.BountyClaimed {
	decimals, err := getTokenDecimals(client, rewardToken)
	if err != nil {
		fmt.Println(err)
		return bountiesClaimed
	}

	block, err := client.BlockByNumber(context.Background(), big.NewInt(int64(blockNumber)))
	if err != nil {
		panic(err)
	}

	timestamp := block.Time()

	historicalPrice := getHistoricalPriceTokenPrice(rewardToken, timestamp)

	bountiesClaimed = append(bountiesClaimed, interfaces.BountyClaimed{
		Contract:        contract,
		TokenReward:     rewardToken,
		Amount:          amount,
		BlockNumber:     blockNumber,
		Timestamp:       timestamp,
		TokenDecimals:   decimals,
		Tx:              txHash,
		HistoricalPrice: historicalPrice,
	})

	return bountiesClaimed
}

func getTokenDecimals(client *ethclient.Client, token common.Address) (uint8, error) {
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

func getTokenPrice(token common.Address) float64 {

	url := "https://coins.llama.fi/prices/current/ethereum:" + token.Hex()
	response, err := http.Get(url)

	// For limit rate
	time.Sleep(1 * time.Second)

	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	defilammaPrice := new(interfaces.DefilammaPrice)
	if err := json.Unmarshal(body, defilammaPrice); err != nil {
		return 0
	}

	obj, exists := defilammaPrice.Coins["ethereum:"+token.Hex()]
	if !exists {
		return 0
	}

	return obj.Price
}

func getTokenPriceFromGeckoterminal(token common.Address) float64 {

	response, err := http.Get("https://api.geckoterminal.com/api/v2/networks/eth/tokens/" + token.Hex() + "?include=top_pools")

	if err != nil {
		fmt.Println(err)
		return 0
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	geckoterminal := new(interfaces.Geckoterminal)
	if err := json.Unmarshal(body, geckoterminal); err != nil {
		return 0
	}

	price, err := strconv.ParseFloat(geckoterminal.Data.Attributes.PriceUSD, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return price
}

func getHistoricalPriceTokenPrice(token common.Address, timestamp uint64) float64 {

	url := "https://coins.llama.fi/prices/historical/" + strconv.FormatUint(timestamp, 10) + "/ethereum:" + token.Hex()
	response, err := http.Get(url)

	// For limit rate
	time.Sleep(500 * time.Millisecond)

	if err != nil {
		return 0
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0
	}

	defilammaPrice := new(interfaces.DefilammaPrice)
	if err := json.Unmarshal(body, defilammaPrice); err != nil {
		return 0
	}

	obj, exists := defilammaPrice.Coins["ethereum:"+token.Hex()]
	if !exists {
		return 0
	}

	return obj.Price
}
