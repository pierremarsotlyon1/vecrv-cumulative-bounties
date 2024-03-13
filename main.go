package main

import (
	"context"
	"encoding/json"
	"log"
	"main/interfaces"
	"main/src"
	"main/utils"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

var ALCHEMY_RPC_URL = ""
var RPC_LOCAL_NODE = "/datastore/.ethereum/geth.ipc"

// json files

const CONFIG_PATH = "./config.json"

// use godot package to load/read the .env file and
// return the value of the key
func goDotEnvVariable(key string) string {

	// load .env file
	godotenv.Load()

	return os.Getenv(key)
}

func main() {

	alchemyApiKey := goDotEnvVariable("ALCHEMY_APIKEY")
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

	src.FetchBounties(client, currentBlock, config, ALCHEMY_RPC_URL)
	src.FetchLocks(client, currentBlock, config)
	src.FetchVotes(client, currentBlock, config)

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
