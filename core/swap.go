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
	usdcAmount = big.NewInt(0).Mul(big.NewInt(100), usdcDecimal)       // 100 usdc
	ethAmount = big.NewInt(0).Mul(
		big.NewInt(0).Div(ethDecimal, big.NewInt(10000000000)),
		big.NewInt(2),
	) // 2*1e-10 eth，与 https://github.com/chainx-org/SoOmnichain/blob/main/scripts/swap.py 测试数据相同
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
	// stargate 跨链仅支持 usdc usdt stargate
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
		// 发交易前需要重新生成
		// dstSwap 的 fromAmount 填 0 即可，合约会自动填入
		dstSwapData, dstUniswapPath, err = createSwapData(toChainInfo, toChainInfo.Usdc, toTokenAddress, big.NewInt(0), big.NewInt(0))
		if err != nil {
			return err
		}
	}

	// 1. 估算目标链交易需要的 dst gas fee，此手续费用来计算 stargate 跨链的总体手续费
	dstGasUint64, err := estimateForGas(toChainInfo, soData, dstSwapData)
	if err != nil {
		return err
	}
	dstGas := big.NewInt(int64(dstGasUint64))
	display.PrintfWithTime("sgReceive 预估手续费：%s\n", dstGas)
	stargateData := newStargateData(fromChainInfo, toChainInfo, big.NewInt(0), dstGas)

	// 从源链获取 stargate cross fee，并计算发给 sodiamond 的 value
	// 2. 预估最终得到的 final amount
	finalAmount, err := estimateFinalAmount(fromChainInfo, testAmount, srcUniswapPath, stargateData, toChainInfo, dstUniswapPath)
	if err != nil {
		return err
	}

	// 3. 根据滑点预估 stargate 发送到目标链的 min amount，并重新构造 dstSwapData
	minAmount, stargateMinAmount, err := estimateMinAmount(toChainInfo, finalAmount, 0.005, dstUniswapPath)
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

	// 4. 发送交易
	if fromTokenAddress != zeroAddress {
		// 4.1 如果 from token 是 erc20，则需要先 approve
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
	// 4.1 计算 stargateFee，跟 value 相加作为最后发送的 value
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
	// 获取当前执行环境
	chainInfo, fromTokenAddress, testAmount, err := getChainAndToken(chain, fromToken)
	if err != nil {
		return err
	}
	_, toTokenAddress, _, err := getChainAndToken(chain, toToken)
	if err != nil {
		return err
	}

	// 构造基本的数据结构
	soData := newSoData(account.Address(), chainInfo.ChainId, fromTokenAddress, chainInfo.ChainId, toTokenAddress, testAmount)
	// 构造 uniswapPath，生产环境下应按照 pair 库存寻找最佳路径
	_, uniswapPath, err := createSwapData(chainInfo, fromTokenAddress, toTokenAddress, testAmount, big.NewInt(0))
	if err != nil {
		return err
	}
	txSendValue := big.NewInt(0)
	if fromTokenAddress == zeroAddress {
		txSendValue = big.NewInt(0).Add(txSendValue, testAmount)
	}

	// 1. 根据滑点计算 minAmount，构造 swapData
	_, amountMinOut, err := estimateUniswapAmount(chainInfo, testAmount, 0.005, uniswapPath)
	if err != nil {
		return err
	}
	swapData, _, err := createSwapData(chainInfo, fromTokenAddress, toTokenAddress, testAmount, amountMinOut)
	if err != nil {
		return err
	}

	// 2. 如果 from token 是 erc20，需要先 approve
	if fromTokenAddress != zeroAddress {
		// 2.1 如果 from token 是 erc20，则需要先 approve
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

	// 3. 调用 sodiamond 合约 swapTokensGeneric
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

// swapTokensGeneric 调用 soDiamond 合约，完成单链 swap
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

// soSwapViaStargate 调用 soDiamond 合约，通过 stargate 跨链兑换
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

// estimateUniswapAmount 估算此路径下 uniswap amountOut amountMinOut
func estimateUniswapAmount(chainInfo Chain, amountIn *big.Int, slippage float32, path []common.Address) (*big.Int, *big.Int, error) {
	pool := getConnectPool(chainInfo.Rpc)
	var err error
	var amountOut *big.Int
	var amountMinOut *big.Int
	err = pool.Call(func(c1 *ethclient.Client, c2 *rpc.Client) error {
		amountsOut, err := newUnisapV2Contract(common.HexToAddress(chainInfo.Swap[0][0])).GetAmountsOut(c1, amountIn, path)
		if err != nil {
			return err
		}
		amountOut = amountsOut[len(amountsOut)-1]
		amountMinOut = decimal.NewFromBigInt(amountOut, 0).Mul(decimal.NewFromFloat32(1.0 - slippage)).BigInt()
		return nil
	})
	return amountOut, amountMinOut, err
}

// estimateMinAmount 根据滑点预估最终得到的最小 amount
// 返回值：目标 token 最小 amount，stargate 发给目标链的最小 amount
func estimateMinAmount(toChainInfo Chain, finalAmount *big.Int, slippage float32, dstPath []common.Address) (*big.Int, *big.Int, error) {
	dstTokenMinAmount := decimal.NewFromBigInt(finalAmount, 0).Mul(decimal.NewFromFloat32(1.0 - slippage)).BigInt()
	stargateMinOut := big.NewInt(0)
	var err error
	pool := getConnectPool(toChainInfo.Rpc)
	if len(dstPath) > 0 {
		err = pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
			amountsIn, err := newUnisapV2Contract(common.HexToAddress(toChainInfo.Swap[0][0])).GetAmountsIn(c1, dstTokenMinAmount, dstPath)
			if err != nil {
				return err
			}
			stargateMinOut, err = newDiamondContract(common.HexToAddress(toChainInfo.SoDiamond)).GetAmountBeforeSoFee(c1, amountsIn[0])
			return err
		})
		if err != nil {
			return nil, nil, err
		}

		// bsc-test usdc 精度是 18，对于 stargate 输出金额（在 from 链上使用时），需要改回精度 6
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

// estimateFinalAmount 预估在没有滑点的情况下，最终能得到的 amount
func estimateFinalAmount(fromChainInfo Chain, amount *big.Int, srcPath []common.Address, stargateData StargateData, toChainInfo Chain, dstPath []common.Address) (*big.Int, error) {
	// 1. 如果 srcPath 不为空，则先根据 uniswap 得到源链的 amount out
	stargateInAmount := amount
	var err error
	// 1. 如果源链需要 swap，先预估 swap 得到的结果
	srcPool := getConnectPool(fromChainInfo.Rpc)
	if len(srcPath) > 0 {
		// 源链 uniswap 合约估算 amount out
		err = srcPool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
			amountsOut, err := newUnisapV2Contract(common.HexToAddress(fromChainInfo.Swap[0][0])).GetAmountsOut(c1, amount, srcPath)
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

	// 2. 预估 stargate 跨链得到的结果
	stargateOutAmount := big.NewInt(0)
	err = srcPool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		// 2.1 计算跨链结果
		diamondContract := newDiamondContract(common.HexToAddress(fromChainInfo.SoDiamond))
		stargateOutAmount, err = diamondContract.EstimateStargateFinalAmount(c1, stargateData, stargateInAmount)
		if err != nil {
			return err
		}
		// 2.2 计算 so fee
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

	// 3. 如果目标链需要 swap，则预估目标链 swap 结果
	dstAmountOut := big.NewInt(0)
	dstPool := getConnectPool(toChainInfo.Rpc)
	err = dstPool.Call(func(c1 *ethclient.Client, c2 *rpc.Client) error {
		// bsc-test net, usdt 精度是 18
		if toChainInfo.Name == "bsc-test" {
			stargateOutAmount = changeDecimals(stargateOutAmount, 6, 18)
		}
		dstAmountsOut, err := newUnisapV2Contract(common.HexToAddress(toChainInfo.Swap[0][0])).
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

// estimateForGas 预估目标链的 gas，此为手续费的一项
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
