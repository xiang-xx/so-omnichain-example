package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"so-omnichain-example/display"
	"time"

	"github.com/coming-chat/wallet-SDK/core/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/fatih/color"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

const (
	zeroAddress         = "0x0000000000000000000000000000000000000000"
	zeroAddressNoPrefix = "0000000000000000000000000000000000000000"
)

var (
	usdcDecimal *big.Int
	ethDecimal  *big.Int
	usdcAmount  *big.Int
	ethAmount   *big.Int
)

var config Config
var account *eth.Account

func initConfig() {
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
}

func init() {
	initConfig()

	usdcDecimal, _ = big.NewInt(0).SetString("1000000", 10)            // 1e6
	ethDecimal, _ = big.NewInt(0).SetString("1000000000000000000", 10) // 1e18
	usdcAmount = big.NewInt(0).Mul(big.NewInt(10), usdcDecimal)        // 10 usdc
	ethAmount = big.NewInt(0).Mul(
		big.NewInt(0).Div(ethDecimal, big.NewInt(100)),
		big.NewInt(2),
	) // 2*1e-2
}

func Swap(fromChain, toChain, fromToken, toToken string) error {
	if fromChain == toChain {
		return swapSameChain(fromChain, fromToken, toToken)
	}
	return swapDiffChain(fromChain, toChain, fromToken, toToken)
}

func swapDiffChain(fromChain, toChain, fromToken, toToken string) error {
	txSendValue := big.NewInt(0)
	fromChainInfo, fromTokenAddress, testAmount, err := getChainAndToken(fromChain, fromToken)
	if err != nil {
		return err
	}
	toChainInfo, toTokenAddress, _, err := getChainAndToken(toChain, toToken)
	if err != nil {
		return err
	}
	soData := newSoData(account.Address(), fromChainInfo.ChainId, fromTokenAddress, toChainInfo.ChainId, toTokenAddress, testAmount)
	srcSwapData := make([]SwapData, 0)
	var srcUniswapPath []common.Address
	dstSwapData := make([]SwapData, 0)
	var dstUniswapPath []common.Address
	// stargate ??????????????? usdc usdt stargate
	if fromTokenAddress != fromChainInfo.Usdc {
		srcSwapData, srcUniswapPath, err = createSwapData(fromChainInfo, fromTokenAddress, fromChainInfo.Usdc, testAmount, big.NewInt(0))
		if err != nil {
			return err
		}
	}
	if fromTokenAddress == zeroAddress {
		txSendValue = big.NewInt(0).Add(txSendValue, testAmount)
	}
	if toTokenAddress != toChainInfo.Usdc {
		// ??????????????????????????????
		// dstSwap ??? fromAmount ??? 0 ??????????????????????????????
		dstSwapData, dstUniswapPath, err = createSwapData(toChainInfo, toChainInfo.Usdc, toTokenAddress, big.NewInt(0), big.NewInt(0))
		if err != nil {
			return err
		}
	}

	// 1. ?????????????????????????????? dst gas fee??????????????????????????? stargate ????????????????????????
	dstGasUint64, err := estimateForGas(toChainInfo, soData, dstSwapData)
	if err != nil {
		return err
	}
	dstGas := big.NewInt(int64(dstGasUint64))
	display.PrintfWithTime("sgReceive ??????????????????%s\n", dstGas)
	stargateData := newStargateData(fromChainInfo, toChainInfo, big.NewInt(0), dstGas)

	// ??????????????? stargate cross fee?????????????????? sodiamond ??? value
	// 2. ????????????????????? final amount
	finalAmount, err := estimateFinalAmount(fromChainInfo, testAmount, srcUniswapPath, stargateData, toChainInfo, dstUniswapPath)
	if err != nil {
		return err
	}

	// 3. ?????????????????? stargate ????????????????????? min amount?????????????????? dstSwapData
	slippage := 0.01
	minAmount, stargateMinAmount, err := estimateMinAmount(toChainInfo, finalAmount, float32(slippage), dstUniswapPath)
	if err != nil {
		return err
	}
	stargateData.MinAmount = stargateMinAmount
	display.PrintfWithTime("amountOut: %s  amountMinOut: %s\n", finalAmount, minAmount)
	display.PrintfWithTime("stargate min amount: %s\n", stargateData.MinAmount)
	if toTokenAddress != toChainInfo.Usdc {
		dstSwapData, _, err = createSwapData(toChainInfo, toChainInfo.Usdc, toTokenAddress, big.NewInt(0), minAmount)
		if err != nil {
			return err
		}
	}

	// 4. ????????????
	if fromTokenAddress != zeroAddress {
		// 4.1 ?????? from token ??? erc20??????????????? approve
		approvedTxHash, err := approve(fromChainInfo, fromTokenAddress, fromChainInfo.SoDiamond, testAmount)
		if err != nil {
			return err
		}
		if approvedTxHash == "" {
			return errors.New("approve failed")
		}
		err = waitForTxSuccess(fromChainInfo.Rpc, approvedTxHash)
		if err != nil {
			return err
		}
	}
	// 4.1 ?????? stargateFee?????? value ??????????????????????????? value
	stargateFee, err := getStargateFee(fromChainInfo, soData, stargateData, dstSwapData)
	if err != nil {
		return err
	}
	display.PrintfWithTime("get stargate fee: %s eth\n", decimal.NewFromBigInt(stargateFee, 0).Div(decimal.NewFromBigInt(ethDecimal, 0)).StringFixed(8))
	txSendValue = big.NewInt(0).Add(txSendValue, stargateFee)

	soData.print()
	stargateData.print()
	fmt.Println("===========================================================")
	fmt.Printf("value:            %s\n", txSendValue)
	txHash, err := soSwapViaStargate(fromChainInfo, soData, srcSwapData, stargateData, dstSwapData, txSendValue)
	if err != nil {
		return err
	}
	display.PrintfWithTime("txHash: %s\n", txHash)
	err = waitForTxSuccess(fromChainInfo.Rpc, txHash)
	if err != nil {
		return err
	}

	return nil
}

func swapSameChain(chain, fromToken, toToken string) error {
	if fromToken == toToken {
		return nil
	}
	// ????????????????????????
	chainInfo, fromTokenAddress, testAmount, err := getChainAndToken(chain, fromToken)
	if err != nil {
		return err
	}
	_, toTokenAddress, _, err := getChainAndToken(chain, toToken)
	if err != nil {
		return err
	}

	// ???????????????????????????
	soData := newSoData(account.Address(), chainInfo.ChainId, fromTokenAddress, chainInfo.ChainId, toTokenAddress, testAmount)
	// ?????? uniswapPath??????????????????????????? pair ????????????????????????
	_, uniswapPath, err := createSwapData(chainInfo, fromTokenAddress, toTokenAddress, testAmount, big.NewInt(0))
	if err != nil {
		return err
	}
	txSendValue := big.NewInt(0)
	if fromTokenAddress == zeroAddress {
		txSendValue = big.NewInt(0).Add(txSendValue, testAmount)
	}

	// 1. ?????????????????? minAmount????????? swapData
	_, amountMinOut, err := estimateUniswapAmount(chainInfo, testAmount, 0.005, uniswapPath)
	if err != nil {
		return err
	}
	swapData, _, err := createSwapData(chainInfo, fromTokenAddress, toTokenAddress, testAmount, amountMinOut)
	if err != nil {
		return err
	}

	// 2. ?????? from token ??? erc20???????????? approve
	if fromTokenAddress != zeroAddress {
		// 2.1 ?????? from token ??? erc20??????????????? approve
		approvedTxHash, err := approve(chainInfo, fromTokenAddress, chainInfo.SoDiamond, testAmount)
		if err != nil {
			return err
		}
		if approvedTxHash == "" {
			return errors.New("approve failed")
		}
		err = waitForTxSuccess(chainInfo.Rpc, approvedTxHash)
		if err != nil {
			return err
		}
	}

	// 3. ?????? sodiamond ?????? swapTokensGeneric
	txHash, err := swapTokensGeneric(chainInfo, soData, swapData, txSendValue)
	if err != nil {
		return err
	}
	display.PrintfWithTime("txHash: %s\n", txHash)
	err = waitForTxSuccess(chainInfo.Rpc, txHash)
	if err != nil {
		return err
	}

	return nil
}

func createSwapData(chainInfo Chain, fromTokenAddress, toTokenAddress string, fromAmount, minAmount *big.Int) ([]SwapData, []common.Address, error) {
	swapItem, path, err := newSwapData(chainInfo, fromTokenAddress, toTokenAddress, fromAmount, minAmount)
	if err != nil {
		return nil, nil, err
	}
	return []SwapData{swapItem}, path, nil
}

// swapTokensGeneric ?????? soDiamond ????????????????????? swap
func swapTokensGeneric(chain Chain, soData SoData, srcSwapDataList []SwapData, value *big.Int) (string, error) {
	pool := getConnectPool(chain.Rpc)
	var err error
	var txHash string
	err = pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		txHash, err = newDiamondContract(common.HexToAddress(chain.SoDiamond)).
			SwapTokensGeneric(chain.Rpc, c1, account, soData, srcSwapDataList, value)
		return err
	})
	return txHash, err
}

// soSwapViaStargate ?????? soDiamond ??????????????? stargate ????????????
func soSwapViaStargate(srcChain Chain, soData SoData, srcSwapDataList []SwapData, stargateData StargateData, dstSwapDataList []SwapData, value *big.Int) (string, error) {
	pool := getConnectPool(srcChain.Rpc)
	var err error
	var txHash string
	err = pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		txHash, err = newDiamondContract(common.HexToAddress(srcChain.SoDiamond)).
			SoSwapViaStargate(srcChain.Rpc, c1, account, soData, srcSwapDataList, stargateData, dstSwapDataList, value)
		return err
	})
	return txHash, err
}

func getStargateFee(chain Chain, soData SoData, stargateData StargateData, swapDataList []SwapData) (*big.Int, error) {
	pool := getConnectPool(chain.Rpc)
	var result *big.Int
	var err error
	err = pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		result, err = newDiamondContract(common.HexToAddress(chain.SoDiamond)).
			GetStargateFee(c1, soData, stargateData, swapDataList)
		return err
	})
	return result, err
}

func approve(chain Chain, tokenAddress string, approveTo string, amount *big.Int) (result string, err error) {
	pool := getConnectPool(chain.Rpc)
	pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		result, err = newErc20Contract(common.HexToAddress(tokenAddress)).Approve(chain.Rpc, c1, account, common.HexToAddress(approveTo), amount)
		return err
	})

	if err == nil {
		fmt.Println("===========================================================")
		fmt.Println("approve to token:")
		fmt.Printf("token:  %s\n", tokenAddress)
		fmt.Printf("to:     %s\n", chain.SoDiamond)
		fmt.Printf("amount: %s\n", amount)
		fmt.Printf("hash:   %s\n", result)
	}
	return
}

func waitForTxSuccess(rpcStr string, txHash string) error {
	pool := getConnectPool(rpcStr)
	ctx := context.Background()
	hashObj := common.HexToHash(txHash)
	isPending := true
	var err error
	success := false

	fmt.Println(color.HiYellowString("wait tx %s", txHash))
	for isPending {
		time.Sleep(time.Second * 3)
		pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
			_, isPending, err = c1.TransactionByHash(ctx, hashObj)
			if err != nil {
				return err
			}
			receipt, err := c1.TransactionReceipt(ctx, hashObj)
			if err != nil {
				return err
			}
			if receipt.Status == 0 {
				success = false
			} else {
				success = true
			}
			return nil
		})
	}
	if success {
		fmt.Println(color.HiGreenString("tx success %s", txHash))
		return nil
	} else {
		fmt.Println(color.HiRedString("tx failed %s", txHash))
	}
	return errors.New("transaction failed:" + txHash)
}

// estimateUniswapAmount ?????????????????? uniswap amountOut amountMinOut
func estimateUniswapAmount(chainInfo Chain, amountIn *big.Int, slippage float32, path []common.Address) (*big.Int, *big.Int, error) {
	pool := getConnectPool(chainInfo.Rpc)
	var err error
	var amountOut *big.Int
	var amountMinOut *big.Int

	swapVersion := versionV2
	quoteAdderss := ""
	if chainInfo.Swap[0][1] == "ISwapRouter" {
		swapVersion = versionV3
		quoteAdderss = chainInfo.Swap[0][2]
	}

	err = pool.Call(func(c1 *ethclient.Client, c2 *rpc.Client) error {
		amountsOut, err := newUnisapV2Contract(common.HexToAddress(chainInfo.Swap[0][0]), swapVersion, quoteAdderss).
			GetAmountsOut(c1, amountIn, path)
		if err != nil {
			return err
		}
		amountOut = amountsOut[len(amountsOut)-1]
		amountMinOut = decimal.NewFromBigInt(amountOut, 0).Mul(decimal.NewFromFloat32(1.0 - slippage)).BigInt()
		return nil
	})
	return amountOut, amountMinOut, err
}

// estimateMinAmount ??????????????????????????????????????? amount
// ?????????????????? token ?????? amount???stargate ???????????????????????? amount
func estimateMinAmount(toChainInfo Chain, finalAmount *big.Int, slippage float32, dstPath []common.Address) (*big.Int, *big.Int, error) {
	dstTokenMinAmount := decimal.NewFromBigInt(finalAmount, 0).Mul(decimal.NewFromFloat32(1.0 - slippage)).BigInt()
	stargateMinOut := big.NewInt(0)
	var err error
	pool := getConnectPool(toChainInfo.Rpc)
	swapVersion := versionV2
	quoteAdderss := ""
	if toChainInfo.Swap[0][1] == "ISwapRouter" {
		swapVersion = versionV3
		quoteAdderss = toChainInfo.Swap[0][2]
	}
	if len(dstPath) > 0 {
		err = pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
			amountsIn, err := newUnisapV2Contract(common.HexToAddress(toChainInfo.Swap[0][0]), swapVersion, quoteAdderss).GetAmountsIn(c1, dstTokenMinAmount, dstPath)
			if err != nil {
				return err
			}
			stargateMinOut, err = newDiamondContract(common.HexToAddress(toChainInfo.SoDiamond)).GetAmountBeforeSoFee(c1, amountsIn[0])
			return err
		})
		if err != nil {
			return nil, nil, err
		}

		// bsc-test usdc ????????? 18????????? stargate ?????????????????? from ??????????????????????????????????????? 6
		if toChainInfo.Name == "bsc-test" {
			stargateMinOut = changeDecimals(stargateMinOut, 18, 6)
		}
	} else {
		err = pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
			stargateMinOut, err = newDiamondContract(common.HexToAddress(toChainInfo.SoDiamond)).GetAmountBeforeSoFee(c1, dstTokenMinAmount)
			return err
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return dstTokenMinAmount, stargateMinOut, nil
}

// estimateFinalAmount ?????????????????????????????????????????????????????? amount
func estimateFinalAmount(fromChainInfo Chain, amount *big.Int, srcPath []common.Address, stargateData StargateData, toChainInfo Chain, dstPath []common.Address) (*big.Int, error) {
	// 1. ?????? srcPath ???????????????????????? uniswap ??????????????? amount out
	stargateInAmount := amount
	var err error
	// 1. ?????????????????? swap???????????? swap ???????????????
	srcPool := getConnectPool(fromChainInfo.Rpc)
	if len(srcPath) > 0 {
		swapVersion := versionV2
		quoteAdderss := ""
		if fromChainInfo.Swap[0][1] == "ISwapRouter" {
			swapVersion = versionV3
			quoteAdderss = fromChainInfo.Swap[0][2]
		}
		// ?????? uniswap ???????????? amount out
		err = srcPool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
			amountsOut, err := newUnisapV2Contract(common.HexToAddress(fromChainInfo.Swap[0][0]), swapVersion, quoteAdderss).GetAmountsOut(c1, amount, srcPath)
			if err != nil {
				return err
			}
			stargateInAmount = amountsOut[len(amountsOut)-1]
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// 2. ?????? stargate ?????????????????????
	stargateOutAmount := big.NewInt(0)
	err = srcPool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		// 2.1 ??????????????????
		diamondContract := newDiamondContract(common.HexToAddress(fromChainInfo.SoDiamond))
		stargateOutAmount, err = diamondContract.EstimateStargateFinalAmount(c1, stargateData, stargateInAmount)
		if err != nil {
			return err
		}
		// 2.2 ?????? so fee
		soFee, err := diamondContract.GetSoFee(c1, stargateOutAmount)
		if err != nil {
			return err
		}
		stargateOutAmount = big.NewInt(0).Sub(stargateOutAmount, soFee)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(dstPath) == 0 {
		return stargateOutAmount, nil
	}

	// 3. ????????????????????? swap????????????????????? swap ??????
	dstAmountOut := big.NewInt(0)
	dstPool := getConnectPool(toChainInfo.Rpc)
	err = dstPool.Call(func(c1 *ethclient.Client, c2 *rpc.Client) error {
		// bsc-test net, usdt ????????? 18
		if toChainInfo.Name == "bsc-test" {
			stargateOutAmount = changeDecimals(stargateOutAmount, 6, 18)
		}
		swapVersion := versionV2
		quoteAdderss := ""
		if toChainInfo.Swap[0][1] == "ISwapRouter" {
			swapVersion = versionV3
			quoteAdderss = toChainInfo.Swap[0][2]
		}
		dstAmountsOut, err := newUnisapV2Contract(common.HexToAddress(toChainInfo.Swap[0][0]), swapVersion, quoteAdderss).
			GetAmountsOut(c1, stargateOutAmount, dstPath)
		if err != nil {
			return err
		}
		dstAmountOut = dstAmountsOut[len(dstAmountsOut)-1]
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dstAmountOut, nil
}

// estimateForGas ?????????????????? gas???????????????????????????
func estimateForGas(toChainInfo Chain, soData SoData, toChainSwapData []SwapData) (uint64, error) {
	var gasRes uint64
	soDiamond := common.HexToAddress(toChainInfo.SoDiamond)
	stargatePoolId := big.NewInt(int64(toChainInfo.StargetaPoolId))
	pool := getConnectPool(toChainInfo.Rpc)
	pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		gas, err := newDiamondContract(soDiamond).SgReceiveForGas(c1, soData, stargatePoolId, toChainSwapData)
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
	case "avax-test":
		return config.Networks.AvaxTest, nil
	case "optimism-test":
		return config.Networks.OptimismTest, nil
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
