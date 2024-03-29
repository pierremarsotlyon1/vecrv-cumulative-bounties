package src

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"main/utils"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

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

const DATA_PATH = "./data.json"
const STATS_PATH = "./stats.json"

func FetchBounties(wg *sync.WaitGroup, client *ethclient.Client, currentBlock uint64, config interfaces.Config, alchemyRpcUrl string) {
	defer wg.Done()

	// Bounties
	allClaimed := make([]interfaces.BountyClaimed, 0)

	previousClaims := readDataPath(DATA_PATH)
	allClaimed = append(allClaimed, previousClaims...)

	fmt.Println("Fetching votium")
	allClaimed = append(allClaimed, fetchVotium(client, currentBlock, config)...)

	fmt.Println("Fetching votemarket v1")
	allClaimed = append(allClaimed, fetchVotemarketV1(client, currentBlock, config)...)

	fmt.Println("Fetching votemarket v2")
	allClaimed = append(allClaimed, fetchVotemarketV2(client, currentBlock, config)...)

	fmt.Println("Fetching quest")
	allClaimed = append(allClaimed, fetchQuest(client, currentBlock, config)...)

	fmt.Println("Fetching yBribe")
	allClaimed = append(allClaimed, fetchYBribe(client, currentBlock, config, alchemyRpcUrl)...)

	fmt.Println("Fetching bribe crv finance")
	allClaimed = append(allClaimed, fetchBribeCrvFinance(client, currentBlock, config, alchemyRpcUrl)...)

	fmt.Println(len(allClaimed), " claims found")

	// Compute stats files
	computeBountiesStats(allClaimed)
}

func computeBountiesStats(allClaimed []interfaces.BountyClaimed) {
	totalBountiesUSD := 0.0
	totalVeCRVBountiesUSD := 0.0
	totalVlCVXBountiesUSD := 0.0

	allClaimedWithPrice := make([]interfaces.BountyClaimed, 0)
	for _, bounty := range allClaimed {

		amount, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(bounty.Amount), big.NewFloat(0).SetInt(math.BigPow(10, int64(bounty.TokenDecimals)))).Float64()
		bountyAmount := (amount * bounty.HistoricalPrice)

		totalBountiesUSD += bountyAmount
		if isVlCVX(bounty) {
			totalVlCVXBountiesUSD += bountyAmount
		} else {
			totalVeCRVBountiesUSD += bountyAmount
		}

		allClaimedWithPrice = append(allClaimedWithPrice, bounty)
	}

	writeDataPath(DATA_PATH, allClaimedWithPrice)

	fmt.Println("Total bounties USD (veCRV + vlCVX): ", fmt.Sprintf("%f", totalBountiesUSD))
	fmt.Println("Total veCRV bounties USD : ", fmt.Sprintf("%f", totalVeCRVBountiesUSD))
	fmt.Println("Total vlCVX bounties USD : ", fmt.Sprintf("%f", totalVlCVXBountiesUSD))

	// Sort claims by timestamp ASC
	sort.Slice(allClaimedWithPrice, func(i, j int) bool { return allClaimedWithPrice[i].Timestamp < allClaimedWithPrice[j].Timestamp })
	timestampEnd := allClaimedWithPrice[len(allClaimedWithPrice)-1].Timestamp
	timestampStart := allClaimedWithPrice[0].Timestamp

	var stats interfaces.Stats

	stats.TotalClaimed = totalBountiesUSD
	stats.VeCRVTotalClaimed = totalVeCRVBountiesUSD
	stats.VlCVXTotalClaimed = totalVlCVXBountiesUSD

	stats.ClaimsLast7Days = generateDaysData(allClaimedWithPrice, timestampEnd-7*utils.DAY_TO_SEC, timestampEnd, 25*utils.MIN_TO_SEC)
	stats.ClaimsLast30Days = generateDaysData(allClaimedWithPrice, timestampEnd-30*utils.DAY_TO_SEC, timestampEnd, 2*utils.HOUR_TO_SEC)
	stats.ClaimsLast180Days = generateDaysData(allClaimedWithPrice, timestampEnd-180*utils.DAY_TO_SEC, timestampEnd, 12*utils.HOUR_TO_SEC)
	stats.ClaimsLast365Days = generateDaysData(allClaimedWithPrice, timestampEnd-365*utils.DAY_TO_SEC, timestampEnd, utils.DAY_TO_SEC)
	stats.ClaimsSinceInception = generateDaysData(allClaimedWithPrice, timestampStart, timestampEnd, 15*utils.DAY_TO_SEC)

	writeStats(stats)
}

func fetchVotium(client *ethclient.Client, currentBlock uint64, config interfaces.Config) []interfaces.BountyClaimed {

	// veCRV
	from := config.LastBlock
	if from == 0 {
		from = 14730004
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
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
	from = config.LastBlock
	if from == 0 {
		from = 13209937
	}
	query = ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
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
	from = config.LastBlock
	if from == 0 {
		from = 18043767
	}
	query = ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
		Addresses: VOTIUM_VL_CVX_V2_ADDRESSES,
		Topics:    [][]common.Hash{{common.HexToHash("0x7c0c0ef7f1ccead819631ed9c10b0728e76274ee5572b53716ea96e7ec735ffa"), common.HexToHash("0xf7992095989ff6fb8484c32670972fb7c67e943107c3f158013e2b7fb96d1971")}},
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

		if strings.EqualFold(vLog.Topics[0].Hex(), "0x7c0c0ef7f1ccead819631ed9c10b0728e76274ee5572b53716ea96e7ec735ffa") {
			event, err := votiumContract.ParseNewIncentive(vLog)
			if err != nil {
				fmt.Println(err)
				continue
			}
			bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.Token, vLog.BlockNumber, event.Amount, vLog.TxHash, "vlCVX")
		} else {
			event, err := votiumContract.ParseIncreasedIncentive(vLog)
			if err != nil {
				fmt.Println(err)
				continue
			}

			bountiesClaimed = addClaim(client, bountiesClaimed, vLog.Address, event.Token, vLog.BlockNumber, event.Increase, vLog.TxHash, "vlCVX")

		}
	}

	return bountiesClaimed
}

func fetchVotemarketV1(client *ethclient.Client, currentBlock uint64, config interfaces.Config) []interfaces.BountyClaimed {

	from := config.LastBlock
	if from == 0 {
		from = 16376672
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
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

func fetchVotemarketV2(client *ethclient.Client, currentBlock uint64, config interfaces.Config) []interfaces.BountyClaimed {

	from := config.LastBlock
	if from == 0 {
		from = 16376672
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
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

func fetchQuest(client *ethclient.Client, currentBlock uint64, config interfaces.Config) []interfaces.BountyClaimed {

	from := config.LastBlock
	if from == 0 {
		from = 14784921
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
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

func fetchYBribe(client *ethclient.Client, currentBlock uint64, config interfaces.Config, alchemyRpcUrl string) []interfaces.BountyClaimed {

	client2, err := ethclient.Dial(alchemyRpcUrl)
	if err != nil {
		panic(err)
	}

	from := config.LastBlock
	if from == 0 {
		from = 15878262
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
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

func fetchBribeCrvFinance(client *ethclient.Client, currentBlock uint64, config interfaces.Config, alchemyRpcUrl string) []interfaces.BountyClaimed {

	bountiesClaimed := make([]interfaces.BountyClaimed, 0)
	pageKey := ""

	hexLastBlock := fmt.Sprintf("%x", config.LastBlock)
	hexCurrentBlock := fmt.Sprintf("%x", currentBlock)

	for {

		query := `{
			"id": 1,
			"jsonrpc": "2.0",
			"method": "alchemy_getAssetTransfers",
			"params": [
			{
				"fromBlock": "0x` + hexLastBlock + `",
				"toBlock": "0x` + hexCurrentBlock + `",
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

		r, err := http.NewRequest("POST", alchemyRpcUrl, bytes.NewBuffer(body))
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
	decimals, err := utils.GetTokenDecimals(client, rewardToken)
	if err != nil {
		fmt.Println(err)
		return bountiesClaimed
	}

	block, err := client.BlockByNumber(context.Background(), big.NewInt(int64(blockNumber)))
	if err != nil {
		panic(err)
	}

	timestamp := block.Time()

	historicalPrice := utils.GetHistoricalPriceTokenPrice(rewardToken, "ethereum", timestamp)

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

func generateDaysData(allClaimed []interfaces.BountyClaimed, start uint64, end uint64, interval uint64) interfaces.StatsClaim {
	var statsClaim interfaces.StatsClaim
	statsClaim.Claims = make([]interfaces.Claim, 0)

	current := start
	currentTotalBountyDollarAmount := 0.0

	for _, claimed := range allClaimed {

		amount, _ := new(big.Float).Quo(big.NewFloat(0).SetInt(claimed.Amount), big.NewFloat(0).SetInt(math.BigPow(10, int64(claimed.TokenDecimals)))).Float64()
		bountyDollarAmount := (amount * claimed.HistoricalPrice)
		currentTotalBountyDollarAmount += bountyDollarAmount

		if claimed.Timestamp < start {
			continue
		}

		statsClaim.TotalClaimed += bountyDollarAmount
		if isVlCVX(claimed) {
			statsClaim.VlCVXTotalClaimed += bountyDollarAmount
		} else {
			statsClaim.VeCRVTotalClaimed += bountyDollarAmount
		}

		if current+interval < claimed.Timestamp {
			statsClaim.Claims = append(statsClaim.Claims, interfaces.Claim{
				Timestamp: current,
				Value:     currentTotalBountyDollarAmount,
			})

			current += interval
		}
	}

	if currentTotalBountyDollarAmount > 0 {
		statsClaim.Claims = append(statsClaim.Claims, interfaces.Claim{
			Timestamp: current,
			Value:     currentTotalBountyDollarAmount,
		})
	}

	return statsClaim
}

func isVlCVX(bounty interfaces.BountyClaimed) bool {
	return bounty.Comment == "vlCVX"
}

func writeDataPath(fileName string, claimed []interfaces.BountyClaimed) {
	file, err := json.Marshal(claimed)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(fileName, file, 0644); err != nil {
		log.Fatal(err)
	}
}

func readDataPath(fileName string) []interfaces.BountyClaimed {

	if !utils.FileExists(fileName) {
		return make([]interfaces.BountyClaimed, 0)
	}

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

func writeStats(stats interfaces.Stats) {
	file, err := json.Marshal(stats)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(STATS_PATH, file, 0644); err != nil {
		log.Fatal(err)
	}
}
