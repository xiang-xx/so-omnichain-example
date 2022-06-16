package main

import (
	"fmt"
	"math/big"
)

const (
	zeroAddress = "0x0000000000000000000000000000000000000000"
)

var (
	usdcDecimal *big.Int
	ethDecimal  *big.Int
	usdcAmount  *big.Int
	ethAmount   *big.Int
)

func init() {
	usdcDecimal, _ = big.NewInt(0).SetString("1000000", 10)            // 1e6
	ethDecimal, _ = big.NewInt(0).SetString("1000000000000000000", 10) // 1e18
	usdcAmount = big.NewInt(0).Mul(big.NewInt(100), usdcDecimal)       // 100 usdc
	ethAmount = big.NewInt(0).Div(ethDecimal, big.NewInt(100))         // 0.01 eth
}

func swap(fromChain, toChain, fromToken, toToken string) error {
	if fromChain == toChain {
		return swapSameChain(fromChain, fromToken, toToken)
	}
	return swapDiffChain(fromChain, toChain, fromToken, toToken)
}

func swapDiffChain(fromChain, toChain, fromToken, toToken string) error {
	fromChainInfo, fromTokenAddress, testAmount, err := getChainAndToken(fromChain, fromToken)
	if err != nil {
		return err
	}
	toChainInfo, toTokenAddress, _, err := getChainAndToken(toChain, toToken)
	if err != nil {
		return err
	}
	soData := newSoData(account.Address(), fromChainInfo.ChainId, fromTokenAddress, toChainInfo.ChainId, toTokenAddress, testAmount)
	fmt.Printf("%v", soData)
	return nil
}

func swapSameChain(chain, fromToken, toToken string) error {
	if fromToken == toToken {
		return nil
	}
	// todo
	return nil
}

func getChainInfo(chain string) (Chain, error) {
	switch chain {
	case "rinkeby":
		return config.Networks.Rinkeby, nil
	case "polygon-test":
		return config.Networks.PolygonTest, nil
	default:
		return Chain{}, errUnsupportChain
	}
}

func getChainAndToken(chain, token string) (Chain, string, *big.Int, error) {
	chainInfo, err := getChainInfo(chain)
	if err != nil {
		return chainInfo, "", big.NewInt(0), err
	}
	switch token {
	case "usdc":
		return chainInfo, chainInfo.Usdc, usdcAmount, nil
	case "eth":
		return chainInfo, zeroAddress, ethAmount, nil
	case "weth":
		return chainInfo, chainInfo.Weth, ethAmount, nil
	default:
		return chainInfo, "", ethAmount, errUnsupportToken
	}
}
