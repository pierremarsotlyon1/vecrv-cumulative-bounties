package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"main/contracts/erc20"
	"main/contracts/questDistributor"
	"main/contracts/votemarketV1"
	"main/contracts/votemarketV2"
	"main/contracts/votiumVECrv"
	"main/contracts/votiumVLCVXOld"
	"main/contracts/votiumVLCVXV2"
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

var VOTIUM_VE_CRV_ADDRESSES = []common.Address{
	common.HexToAddress("0xB4Fb1FD4AEC780BC255bF231189E9A244475d260"),
}

var VOTIUM_VL_CVX_V1_ADDRESSES = []common.Address{
	common.HexToAddress("0x19BBC3463Dd8d07f55438014b021Fb457EBD4595"),
}

var VOTIUM_VL_CVX_V2_ADDRESSES = []common.Address{
	common.HexToAddress("0x63942E31E98f1833A234077f47880A66136a2D1e"),
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

var BRIBE_CRV_FINANCE_ADDRESSES = []common.Address{
	common.HexToAddress("0x7893bbb46613d7a4fbcc31dab4c9b823ffee1026"),
}

var tokens = make(map[common.Address]uint8, 0)
var tokenPrices = make(map[string]float64, 0)

const RPC_URL = "/datastore/.ethereum/geth.ipc"

const ALCHEMY_APIKEY = ""
const ALCHEMY_RPC_URL = "https://eth-mainnet.g.alchemy.com/v2/" + ALCHEMY_APIKEY

const DATA_PATH = "./data.json"

const VOTIUM_PATH = "./votium.json"
const VOTEMARKET_V1_PATH = "./votium-vm.json"
const VOTEMARKET_V2_PATH = "./votium-vm-vm2.json"
const QUEST_PATH = "./votium-vm-vm2-quest.json"
const YBRIBE_PATH = "./votium-vm-vm2-quest-ybribe.json"
const BRIBE_CRV_FINANCE = "./votium-vm-vm2-quest-bribecrvfinance.json"

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

	if !fileExists(VOTIUM_PATH) {
		fmt.Println("Fetching votium")
		allClaimed = append(allClaimed, fetchVotium(client)...)
		write(VOTIUM_PATH, allClaimed)
	} else {
		allClaimed = read(VOTIUM_PATH)
	}

	if !fileExists(VOTEMARKET_V1_PATH) {
		fmt.Println("Fetching votemarket v1")
		allClaimed = append(allClaimed, fetchVotemarketV1(client)...)
		write(VOTEMARKET_V1_PATH, allClaimed)
	} else {
		allClaimed = read(VOTEMARKET_V1_PATH)
	}

	if !fileExists(VOTEMARKET_V2_PATH) {
		fmt.Println("Fetching votemarket v2")
		allClaimed = append(allClaimed, fetchVotemarketV2(client)...)
		write(VOTEMARKET_V2_PATH, allClaimed)
	} else {
		allClaimed = read(VOTEMARKET_V2_PATH)
	}

	if !fileExists(QUEST_PATH) {
		fmt.Println("Fetching quest")
		allClaimed = append(allClaimed, fetchQuest(client)...)
		write(QUEST_PATH, allClaimed)
	} else {
		allClaimed = read(QUEST_PATH)
	}

	if !fileExists(YBRIBE_PATH) {
		fmt.Println("Fetching yBribe")
		allClaimed = append(allClaimed, fetchYBribe(client)...)
		write(YBRIBE_PATH, allClaimed)
	} else {
		allClaimed = read(YBRIBE_PATH)
	}

	if !fileExists(BRIBE_CRV_FINANCE) {
		fmt.Println("Fetching bribe crv finance")
		allClaimed = append(allClaimed, fetchBribeCrvFinance(client)...)
		write(BRIBE_CRV_FINANCE, allClaimed)
	} else {
		allClaimed = read(BRIBE_CRV_FINANCE)
	}

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

	fmt.Println(len(allClaimed), " claims found")

	totalBountiesUSD := 0.0
	totalVeCRVBountiesUSD := 0.0
	totalVlCVXBountiesUSD := 0.0
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
		bountyAmount := (amount * price)

		totalBountiesUSD += bountyAmount
		if bounty.Comment == "vlCVX" {
			totalVlCVXBountiesUSD += bountyAmount
		} else {
			totalVeCRVBountiesUSD += bountyAmount
		}
	}

	file, err = json.Marshal(allClaimed)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(DATA_PATH, file, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Total bounties USD (veCRV + vlCVX): ", fmt.Sprintf("%f", totalBountiesUSD))
	fmt.Println("Total veCRV bounties USD : ", fmt.Sprintf("%f", totalVeCRVBountiesUSD))
	fmt.Println("Total vlCVX bounties USD : ", fmt.Sprintf("%f", totalVlCVXBountiesUSD))
}

func fetchVotium(client *ethclient.Client) []interfaces.BountyClaimed {

	// veCRV
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(14730004),
		ToBlock:   nil, // Latest
		Addresses: VOTIUM_VE_CRV_ADDRESSES,
		Topics:    [][]common.Hash{{common.HexToHash("0x51c8cd367a987b8c2f652c101ea7076ec8e4dfd33c4c77bb80e018e7143b6512")}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)

	for _, vLog := range logs {
		votiumContract, err := votiumVECrv.NewVotiumVECrv(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		event, err := votiumContract.ParseNewReward(vLog)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.Token, vLog.BlockNumber, event.Amount, vLog.TxHash, "")
	}

	// vlCVX (old)
	query = ethereum.FilterQuery{
		FromBlock: big.NewInt(13209937),
		ToBlock:   nil, // Latest
		Addresses: VOTIUM_VL_CVX_V1_ADDRESSES,
		Topics:    [][]common.Hash{{common.HexToHash("0x74bd0b58587a15767427910140bcf99db1ef7f905cb0a2983a72cd2033954227")}},
	}

	logs, err = client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	for _, vLog := range logs {
		votiumContract, err := votiumVLCVXOld.NewVotiumVLCVXOld(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		event, err := votiumContract.ParseBribed(vLog)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.Token, vLog.BlockNumber, event.Amount, vLog.TxHash, "vlCVX")
	}

	// vlCVX V2
	query = ethereum.FilterQuery{
		FromBlock: big.NewInt(18043767),
		ToBlock:   nil, // Latest
		Addresses: VOTIUM_VL_CVX_V2_ADDRESSES,
		Topics:    [][]common.Hash{{common.HexToHash("0x7c0c0ef7f1ccead819631ed9c10b0728e76274ee5572b53716ea96e7ec735ffa")}},
	}

	logs, err = client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	for _, vLog := range logs {
		votiumContract, err := votiumVLCVXV2.NewVotiumVLCVXV2(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		event, err := votiumContract.ParseNewIncentive(vLog)
		if err != nil {
			fmt.Println(err)
			continue
		}

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.Token, vLog.BlockNumber, event.Amount, vLog.TxHash, "vlCVX")
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

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash, "")
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

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash, "")
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

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash, "")
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

		bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.RewardToken, vLog.BlockNumber, event.Amount, vLog.TxHash, "")
	}

	return bountiesClaimed
}

func fetchBribeCrvFinance(client *ethclient.Client) []interfaces.BountyClaimed {

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)
	pageKey := ""

	for {
		posturl := "https://eth-mainnet.g.alchemy.com/v2/" + ALCHEMY_APIKEY

		query := `{
			"id": 1,
			"jsonrpc": "2.0",
			"method": "alchemy_getAssetTransfers",
			"params": [
			{
				"fromBlock": "0x0",
				"toBlock": "latest",
				"withMetadata": false,
				"excludeZeroValue": true,
				"maxCount": "0x3e8",
				"fromAddress": "0x7893bbb46613d7a4FbcC31Dab4C9b823FfeE1026",
				"category": [
				"erc20"
				],
				"order": "asc"
		`
		if len(pageKey) > 0 {
			query += ",\"pageKey\":\"" + pageKey + "\""
		}

		query += `
				}
				]
			}
		`

		body := []byte(query)

		r, err := http.NewRequest("POST", posturl, bytes.NewBuffer(body))
		if err != nil {
			panic(err)
		}

		r.Header.Add("Content-Type", "application/json")
		r.Header.Add("accept", "application/json")

		httpClient := &http.Client{}
		res, err := httpClient.Do(r)
		if err != nil {
			panic(err)
		}

		defer res.Body.Close()

		post := new(interfaces.AlchemyTransfers)
		derr := json.NewDecoder(res.Body).Decode(post)
		if derr != nil {
			panic(derr)
		}

		if res.StatusCode != http.StatusOK {
			panic(res.Status)
		}

		pageKey = post.Result.PageKey

		for _, transfer := range post.Result.Transfers {
			blockNumber, err := strconv.ParseInt(transfer.BlockNum[2:], 16, 64)
			if err != nil {
				panic(err)
			}

			value, success := big.NewInt(0).SetString(transfer.RawContract.Value[2:], 16)
			if !success {
				panic("Error convert hex")
			}

			bountiesClaimed = addClaim(client, bountiesClaimed, common.HexToAddress(transfer.From), common.HexToAddress(transfer.RawContract.Address), uint64(blockNumber), value, common.HexToHash(transfer.Hash), "")
		}

		if len(pageKey) == 0 {
			break
		}
	}

	return bountiesClaimed
}

func addClaim(client *ethclient.Client, bountiesClaimed []interfaces.BountyClaimed, contract common.Address, rewardToken common.Address, blockNumber uint64, amount *big.Int, txHash common.Hash, comment string) []interfaces.BountyClaimed {
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
		Comment:         comment,
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

	obj, exists := defilammaPrice.Coins["ethereum:"+token.Hex()]
	if !exists {
		tokenPrices[url] = 0
		return 0
	}

	tokenPrices[url] = obj.Price

	return obj.Price
}

func write(fileName string, claimed []interfaces.BountyClaimed) {
	file, err := json.Marshal(claimed)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(fileName, file, 0644); err != nil {
		log.Fatal(err)
	}
}

func fileExists(fileName string) bool {
	if _, err := os.Stat(fileName); err != nil {
		return false
	}

	return true
}

func read(fileName string) []interfaces.BountyClaimed {
	file, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatal(err)
	}

	allClaimed := make([]interfaces.BountyClaimed, 0)
	if err := json.Unmarshal([]byte(file), &allClaimed); err != nil {
		log.Fatal(err)
	}

	return allClaimed
}
