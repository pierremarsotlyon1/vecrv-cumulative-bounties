package main

import (
	"context"
	"encoding/json"
	"log"
	"main/interfaces"
	"main/src"
	"main/utils"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
)

var ALCHEMY_RPC_URL = ""
var RPC_LOCAL_NODE = "/datastore/.ethereum/geth.ipc"

// json files

const CONFIG_PATH = "./config.json"

func main() {

	alchemyApiKey := utils.GoDotEnvVariable("ALCHEMY_APIKEY")
	if len(alchemyApiKey) == 0 {
		panic("ALCHEMY_APIKEY not set")
	}

	ALCHEMY_RPC_URL = "https://eth-mainnet.g.alchemy.com/v2/" + alchemyApiKey

	client, err := ethclient.Dial(RPC_LOCAL_NODE)
	if err != nil {
		client, err = ethclient.Dial(ALCHEMY_RPC_URL)
		if err != nil {
			panic(err)
		}
	}

	currentBlock, err := client.BlockNumber(context.Background())
	if err != nil {
		panic(err)
	}

	config := readConfig()

	var wg sync.WaitGroup
	wg.Add(4)

	go src.FetchBounties(&wg, client, currentBlock, config, ALCHEMY_RPC_URL)
	go src.FetchLocks(&wg, client, currentBlock, config)
	go src.FetchVotes(&wg, client, currentBlock, config)
	go src.FetchGaugeWeights(&wg)

	wg.Wait()

	// Write new config
	config.LastBlock = currentBlock
	writeConfig(config)

}

func readConfig() interfaces.Config {

	if !utils.FileExists(CONFIG_PATH) {
		return interfaces.Config{
			LastBlock: 0,
		}
	}

	file, err := os.ReadFile(CONFIG_PATH)
	if err != nil {
		log.Fatal(err)
	}

	var config interfaces.Config
	if err := json.Unmarshal([]byte(file), &config); err != nil {
		log.Fatal(err)
	}

	return config
}

func writeConfig(config interfaces.Config) {
	file, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(CONFIG_PATH, file, 0644); err != nil {
		log.Fatal(err)
	}
}
