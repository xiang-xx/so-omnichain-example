package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/coming-chat/wallet-SDK/core/eth"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

var config Config
var account *eth.Account
var diamondAbi *abi.ABI
var erc20Abi *abi.ABI

func init() {
	data, err := os.ReadFile("./config.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	// load account
	account, err = eth.NewAccountWithMnemonic(os.Getenv("words"))
	if err != nil {
		panic(err)
	}

	// load abi
	file, err := os.Open("abi/so_diamond.json")
	if err != nil {
		panic(err)
	}
	tmpDiamondAbi, err := abi.JSON(file)
	diamondAbi = &tmpDiamondAbi
	if err != nil {
		panic(err)
	}

	file, err = os.Open("abi/erc20.json")
	if err != nil {
		panic(err)
	}
	tmpErc20Abi, err := abi.JSON(file)
	erc20Abi = &tmpErc20Abi
	if err != nil {
		panic(erc20Abi)
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
