package utils

func GetPublicRpcUrl(chainName string) string {
	switch chainName {
	case "polygon":
		return "https://polygon-mainnet.infura.io/v3/" + GoDotEnvVariable("INFURA_APIKEY")
	case "arbitrum":
		return "https://arbitrum-mainnet.infura.io/v3/" + GoDotEnvVariable("INFURA_APIKEY")
	case "base":
		return "https://mainnet.base.org"
	case "celo":
		return "https://celo-mainnet.infura.io/v3/" + GoDotEnvVariable("INFURA_APIKEY")
	case "optimism":
		return "https://optimism-mainnet.infura.io/v3/" + GoDotEnvVariable("INFURA_APIKEY")
	case "xdai":
		return "https://gnosis-mainnet.public.blastapi.io"
	case "avalanche":
		return "https://avalanche-mainnet.infura.io/v3/" + GoDotEnvVariable("INFURA_APIKEY")
	case "fantom":
		return "https://rpc.fantom.network"
	case "kava":
		return "https://evm.kava.io"
	case "moonbeam":
		return "https://rpc.ankr.com/moonbeam"
	default:
		return ""
	}
}
