package src

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"main/contracts/voter"
	"main/interfaces"
	"main/utils"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var CURVE_OWNERSHIP_VOTER = common.HexToAddress("0xE478de485ad2fe566d49342Cbd03E49ed7DB3356")
var CURVE_PARAMETER_VOTER = common.HexToAddress("0xBCfF8B0b9419b9A88c44546519b1e909cF330399")

var VOTE_CREATE_TOPIC = common.HexToHash("0x0730610a5322c6584fb6f5bb760719e650c888cfd965a2beb2d598bcd425e394")
var VOTE_EXECUTED_TOPIC = common.HexToHash("0xbf8e2b108bb7c980e08903a8a46527699d5e84905a082d56dacb4150725c8cab")
var CAST_VOTE_TOPIC = common.HexToHash("0xb34ee265e3d4f5ec4e8b52d59b2a9be8fceca2f274ebc080d8fba797fea9391f")

const VOTES_PATH = "./votes.json"
const OWNERSHIP = "ownership"
const PARAMETER = "parameter"

func FetchVotes(client *ethclient.Client, currentBlock uint64, config interfaces.Config) {

	fmt.Println("Fetching votes")

	// Read new vote event to fetch ipfs description
	from := config.LastBlock
	if from == 0 {
		from = 10648599
	}
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(from)),
		ToBlock:   big.NewInt(int64(currentBlock)),
		Addresses: []common.Address{CURVE_OWNERSHIP_VOTER, CURVE_PARAMETER_VOTER},
		Topics: [][]common.Hash{{
			VOTE_CREATE_TOPIC,
			VOTE_EXECUTED_TOPIC,
			CAST_VOTE_TOPIC,
		}},
	}

	logs, err := client.FilterLogs(context.Background(), query)
	if err != nil {
		panic(err)
	}

	votes := readVotes()
	fmt.Println("Already ", len(votes), "votes")

	for _, vLog := range logs {
		voterContract, err := voter.NewVoter(vLog.Address, client)
		if err != nil {
			panic(err)
		}

		if strings.EqualFold(vLog.Topics[0].Hex(), VOTE_CREATE_TOPIC.Hex()) {
			event, err := voterContract.ParseStartVote(vLog)
			if err != nil {
				fmt.Println(err)
				continue
			}

			// Check if we didn't already added it
			found := false
			for _, vote := range votes {
				if strings.EqualFold(vote.Voter.Hex(), vLog.Address.Hex()) && event.VoteId.Cmp(vote.Id) == 0 {
					found = true
					break
				}
			}

			if found {
				continue
			}

			// Get vote for extra data
			v, err := voterContract.GetVote(nil, event.VoteId)
			if err != nil {
				panic(err)
			}

			// Fetch voteTime to calculate vote end timestamp
			voteTime, err := voterContract.VoteTime(nil)
			if err != nil {
				panic(err)
			}

			// Check type
			voteType := PARAMETER
			if strings.EqualFold(vLog.Address.Hex(), CURVE_OWNERSHIP_VOTER.Hex()) {
				voteType = OWNERSHIP
			}

			// Check ipfs
			metadata := event.Metadata
			ipfsId := ""
			description := ""
			if strings.Contains(metadata, "ipfs:") {
				ipfsId = metadata[len("ipfs:"):]

				description = utils.GetIpfs(ipfsId)
			}

			votes = append(votes, interfaces.Vote{
				Id:              event.VoteId,
				Open:            v.Open,
				Executed:        v.Executed,
				StartDate:       v.StartDate,
				EndDate:         v.StartDate + voteTime,
				SnapshotBlock:   v.SnapshotBlock,
				SupportRequired: v.SupportRequired,
				MinAcceptQuorum: v.MinAcceptQuorum,
				Yea:             v.Yea,
				Nay:             v.Nay,
				VotingPower:     v.VotingPower,
				Script:          "0x" + hex.EncodeToString(v.Script),
				VoteType:        voteType,
				Voter:           vLog.Address,
				IpfsId:          ipfsId,
				Description:     description,
				Creator:         event.Creator,
			})
		} else if strings.EqualFold(vLog.Topics[0].Hex(), VOTE_EXECUTED_TOPIC.Hex()) || strings.EqualFold(vLog.Topics[0].Hex(), CAST_VOTE_TOPIC.Hex()) {

			// Get vote id according to event
			voteId := big.NewInt(0)
			if strings.EqualFold(vLog.Topics[0].Hex(), VOTE_EXECUTED_TOPIC.Hex()) {
				event, err := voterContract.ParseExecuteVote(vLog)
				if err != nil {
					fmt.Println(err)
					continue
				}
				voteId = event.VoteId
			} else if strings.EqualFold(vLog.Topics[0].Hex(), CAST_VOTE_TOPIC.Hex()) {
				event, err := voterContract.ParseCastVote(vLog)
				if err != nil {
					fmt.Println(err)
					continue
				}
				voteId = event.VoteId
			}

			for i := 0; i < len(votes); i++ {
				vote := votes[i]

				if strings.EqualFold(vote.Voter.Hex(), vLog.Address.Hex()) && voteId.Cmp(vote.Id) == 0 {
					v, err := voterContract.GetVote(nil, voteId)
					if err != nil {
						panic(err)
					}

					votes[i].Executed = v.Executed
					votes[i].Open = v.Open
					votes[i].Nay = v.Nay
					votes[i].Yea = v.Yea

					break
				}
			}
		}
	}

	// Check if we can get missing ipfs description
	for i := 0; i < len(votes); i++ {
		if len(votes[i].Description) == 0 {
			votes[i].Description = utils.GetIpfs(votes[i].IpfsId)
		}
	}

	writeVotes(votes)
}

func readVotes() []interfaces.Vote {

	if !utils.FileExists(VOTES_PATH) {
		return make([]interfaces.Vote, 0)
	}

	file, err := os.ReadFile(VOTES_PATH)
	if err != nil {
		log.Fatal(err)
	}

	votes := make([]interfaces.Vote, 0)
	if err := json.Unmarshal([]byte(file), &votes); err != nil {
		log.Fatal(err)
	}

	return votes
}

func writeVotes(votes []interfaces.Vote) {
	file, err := json.Marshal(votes)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(VOTES_PATH, file, 0644); err != nil {
		log.Fatal(err)
	}
}
