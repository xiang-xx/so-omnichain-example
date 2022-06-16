package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/coming-chat/wallet-SDK/core/eth"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

var config Config
var account *eth.Account

func init() {
	data, err := os.ReadFile("./config.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	account, err = eth.NewAccountWithMnemonic(os.Getenv("words"))
	if err != nil {
		panic(err)
	}
}

func main() {
	var (
		fromChain = flag.String("fc", "rinkeby", "from chain")
		toChain   = flag.String("tc", "polygon-test", "to chain")
		fromToken = flag.String("ft", "usdc", "from token")
		toToken   = flag.String("tt", "usdc", "to token")
	)

	fmt.Println(color.HiBlueString("account: %s", account.Address()))
	fmt.Println(color.HiBlueString("%s %s -->> %s %s", *fromChain, *fromToken, *toChain, *toToken))

	err := swap(*fromChain, *toChain, *fromToken, *toToken)
	if err != nil {
		panic(err)
	}
}
