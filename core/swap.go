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
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/fatih/color"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
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

var config Config
var account *eth.Account
var diamondAbi *abi.ABI
var erc20Abi *abi.ABI

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
	dstGas := big.NewInt(int64(gas))
	display.PrintfWithTime("dst gas for sgReceive %d\n", gas)

	// 如果 from token 是 erc20token，需要先 approve
	if fromTokenAddress != zeroAddress {
		approvedTxHash, err := approve(fromChainInfo, fromTokenAddress, fromChainInfo.SoDiamond, testAmount)
		if err != nil {
			return err
		}
		if approvedTxHash == "" {
			return errors.New("approve failed")
		}
		fmt.Println("===========================================================")
		fmt.Println("approve to token:")
		fmt.Printf("token:  %s\n", fromToken)
		fmt.Printf("to:     %s\n", fromChainInfo.SoDiamond)
		fmt.Printf("amount: %s\n", testAmount)
		fmt.Printf("hash:   %s\n", approvedTxHash)
		err = waitForTxSuccess(fromChainInfo.Rpc, approvedTxHash)
		if err != nil {
			return err
		}
	}

	// 从源链获取 stargate cross fee，并计算发给 sodiamond 的 value
	stargetData := newStargateData(fromChainInfo, toChainInfo, big.NewInt(0), dstGas)
	stargateFee, err := getStargateFee(fromChainInfo, soData, stargetData, make([]SwapData, 0))
	if err != nil {
		return err
	}
	display.PrintfWithTime("get stargate fee: %s eth\n", decimal.NewFromBigInt(stargateFee, 0).Div(decimal.NewFromBigInt(ethDecimal, 0)).StringFixed(8))
	value := stargateFee
	if fromTokenAddress == zeroAddress {
		value = big.NewInt(0).Add(value, testAmount)
	}

	soData.print()
	stargetData.print()
	fmt.Println("===========================================================")
	fmt.Printf("value:            %s\n", value)
	txHash, err := soSwapViaStargate(fromChainInfo, soData, make([]SwapData, 0), stargetData, make([]SwapData, 0), value)
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
	// todo
	return nil
}

func soSwapViaStargate(srcChain Chain, soData SoData, srcSwapDataList []SwapData, stargateData StargateData, dstSwapDataList []SwapData, value *big.Int) (string, error) {
	pool := getConnectPool(srcChain.Rpc)
	var err error
	var txHash string
	err = pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		txHash, err = newDiamondContract(common.HexToAddress(srcChain.SoDiamond), diamondAbi).
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
		result, err = newDiamondContract(common.HexToAddress(chain.SoDiamond), diamondAbi).
			GetStargateFee(c1, soData, stargateData, swapDataList)
		return err
	})
	return result, err
}

func approve(chain Chain, tokenAddress string, approveTo string, amount *big.Int) (result string, err error) {
	pool := getConnectPool(chain.Rpc)
	pool.Call(func(c1 *ethclient.Client, _ *rpc.Client) error {
		result, err = newErc20Contract(common.HexToAddress(tokenAddress), erc20Abi).Approve(chain.Rpc, c1, account, common.HexToAddress(approveTo), amount)
		return err
	})
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

func estimateForGas(toChainInfo Chain, soData SoData, toChainSwapData []SwapData) (uint64, error) {
	var gasRes uint64
	soDiamond := common.HexToAddress(toChainInfo.SoDiamond)
	stargatePoolId := big.NewInt(int64(toChainInfo.StargetaPoolId))
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
