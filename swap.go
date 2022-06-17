package main

import (
	"errors"
	"math/big"
	"so-omnichain-example/display"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	zeroAddress = "0x0000000000000000000000000000000000000000"

	methodSgReceiveForGas = "sgReceiveForGas"
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
	gas, err := estimateForGas(toChainInfo, soData, []SwapData{})
	if err != nil {
		return err
	}
	display.PrintfWithTime("dst gas for sgReceive %d\n", gas)

	// 如果 from token 是 erc20token，需要先 approve
	if fromTokenAddress != zeroAddress {
		approvedTxHash, err := approve(fromChainInfo, fromTokenAddress, testAmount)
		if err != nil {
			return err
		}
		if approvedTxHash == "" {
			return errors.New("approve failed")
		}
		display.PrintfWithTime("approve to token %s, amount %s, txHash: %s", fromTokenAddress, testAmount.String(), approvedTxHash)
	}

	return nil
}

func swapSameChain(chain, fromToken, toToken string) error {
	if fromToken == toToken {
		return nil
	}
	// todo
	return nil
}

func approve(chain Chain, tokenAddress string, amount *big.Int) (result string, err error) {
	pool := getConnectPool(chain.Rpc)
	pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		result, err = newErc20Contract(common.HexToAddress(tokenAddress), erc20Abi).Approve(chain.Rpc, c1, account, amount)
		return err
	})
	return
}

func estimateForGas(toChainInfo Chain, soData SoData, toChainSwapData []SwapData) (uint64, error) {
	var gasRes uint64
	soDiamond := common.HexToAddress(toChainInfo.SoDiamond)
	stargatePoolId, _ := big.NewInt(0).SetString(toChainInfo.StargetaPoolId, 10)
	pool := getConnectPool(toChainInfo.Rpc)
	pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		gas, err := newDiamondContract(soDiamond, diamondAbi).SgReceiveForGas(c1, soData, stargatePoolId, toChainSwapData)
		if err != nil {
			return err
		}
		gasRes = gas
		return nil
	})
	return gasRes, nil
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
