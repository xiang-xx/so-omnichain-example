package core

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/coming-chat/wallet-SDK/core/eth"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
)

const (
	methodApprove                     = "approve"
	methodSgReceiveForGas             = "sgReceiveForGas"
	methodGetStargateFee              = "getStargateFee"
	methodSoSwapViaStargate           = "soSwapViaStargate"
	methodSwapTokensGeneric           = "swapTokensGeneric"
	methodGetAmountsOut               = "getAmountsOut"
	methodGetAmountIn                 = "getAmountsIn"
	methodEstimateStargateFinalAmount = "estimateStargateFinalAmount"
	methodGetSoFee                    = "getSoFee"
	methodGetAmountBeforeSoFee        = "getAmountBeforeSoFee"
	methodExactInput                  = "exactInput" // v3 swap
	methodQuoteExactInput             = "quoteExactInput"
	methodQuoteExactOutput            = "quoteExactOutput"

	versionV2 = "v2"
	versionV3 = "v3"

	AddrSize = 20
	FeeSize  = 3
	Offset   = AddrSize + FeeSize
	DataSize = Offset + AddrSize
)

var (
	diamondAbi     *abi.ABI
	erc20Abi       *abi.ABI
	uniswapEthAbi  *abi.ABI
	uniswapAvaxAbi *abi.ABI
	uniswapV3Abi   *abi.ABI
	quoterAbi      *abi.ABI
)

func init() {
	initAbi(&diamondAbi, "abi/so_diamond.json")
	initAbi(&erc20Abi, "abi/erc20.json")
	initAbi(&uniswapEthAbi, "abi/IUniswapV2Router02.json")
	initAbi(&uniswapAvaxAbi, "abi/IUniswapV2Router02AVAX.json")
	initAbi(&uniswapV3Abi, "abi/ISwapRouter.json")
	initAbi(&quoterAbi, "abi/IQuoter.json")
}

func initAbi(a **abi.ABI, path string) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	tmpAbi, err := abi.JSON(file)
	*a = &tmpAbi
	if err != nil {
		panic(err)
	}
}

type ExactInputParams struct {
	Path             []byte
	Recipient        common.Address
	Deadline         *big.Int
	AmountIn         *big.Int
	AmountOutMinimum *big.Int
}

type baseContract struct {
	Address common.Address
	ChainId *big.Int
	Abi     *abi.ABI
}

type DiamondContract struct {
	baseContract
}

func newDiamondContract(address common.Address) *DiamondContract {
	return &DiamondContract{
		baseContract{
			Address: address,
			Abi:     diamondAbi,
		},
	}
}

func (c *DiamondContract) EstimateStargateFinalAmount(client *ethclient.Client, stargateData StargateData, amount *big.Int) (*big.Int, error) {
	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodEstimateStargateFinalAmount, stargateData, amount)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := big.NewInt(0)
	err = unpackOutput(&resp, c.Abi, methodEstimateStargateFinalAmount, resData)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DiamondContract) GetSoFee(client *ethclient.Client, amount *big.Int) (*big.Int, error) {
	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodGetSoFee, amount)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := big.NewInt(0)
	err = unpackOutput(&resp, c.Abi, methodGetSoFee, resData)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DiamondContract) SgReceiveForGas(client *ethclient.Client, soData SoData, stargatePoolId *big.Int, toChainSwapData []SwapData) (uint64, error) {
	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodSgReceiveForGas, soData, stargatePoolId, toChainSwapData)
	if err != nil {
		return 0, err
	}
	return bind.ContractTransactor(client).EstimateGas(context.Background(), msg)
}

func (c *DiamondContract) GetAmountBeforeSoFee(client *ethclient.Client, amount *big.Int) (*big.Int, error) {
	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodGetAmountBeforeSoFee, amount)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := big.NewInt(0)
	err = unpackOutput(&resp, c.Abi, methodGetAmountBeforeSoFee, resData)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DiamondContract) GetStargateFee(client *ethclient.Client, soData SoData, stargateData StargateData, swapDataList []SwapData) (*big.Int, error) {
	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodGetStargateFee, soData, stargateData, swapDataList)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := big.NewInt(0)
	err = unpackOutput(&resp, c.Abi, methodGetStargateFee, resData)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *DiamondContract) SwapTokensGeneric(rpc string,
	client *ethclient.Client,
	account *eth.Account,
	soData SoData,
	srcSwapDataList []SwapData,
	value *big.Int) (string, error) {
	ctx := context.Background()
	accountAddress := common.HexToAddress(account.Address())
	opts := &bind.TransactOpts{
		From: accountAddress,
	}
	// 合约参数
	msg, err := packInput(c.Abi, opts.From, c.Address, methodSwapTokensGeneric, soData, srcSwapDataList)
	if err != nil {
		return "", err
	}
	// 获取 gas gasprice,构造 tx, encode 成 []byte，使用 account 签名，使用 client 发送
	// 交易数据由服务端构造，sdk 组装成 tx, marshalBinary 之后交给 wallet-sdk 处理签名 + 发送
	rawBytes, err := createRawTx(ctx, client, accountAddress, &c.Address, msg, value)
	if err != nil {
		return "", err
	}
	return signAndSendTx(rawBytes, rpc, account)
}

func (c *DiamondContract) SoSwapViaStargate(rpc string,
	client *ethclient.Client,
	account *eth.Account,
	soData SoData,
	srcSwapDataList []SwapData,
	stargateData StargateData,
	dstSwapDataList []SwapData,
	value *big.Int) (string, error) {
	ctx := context.Background()
	accountAddress := common.HexToAddress(account.Address())
	opts := &bind.TransactOpts{
		From: accountAddress,
	}
	// 合约参数
	msg, err := packInput(c.Abi, opts.From, c.Address, methodSoSwapViaStargate, soData, srcSwapDataList, stargateData, dstSwapDataList)
	if err != nil {
		return "", err
	}
	// 获取 gas gasprice,构造 tx, encode 成 []byte，使用 account 签名，使用 client 发送
	// 交易数据由服务端构造，sdk 组装成 tx, marshalBinary 之后交给 wallet-sdk 处理签名 + 发送
	rawBytes, err := createRawTx(ctx, client, accountAddress, &c.Address, msg, value)
	if err != nil {
		return "", err
	}
	return signAndSendTx(rawBytes, rpc, account)
}

type UniswapV2Contract struct {
	baseContract
	swapVersion  string
	quoteAbi     *abi.ABI
	quoteAddress common.Address
}

func newUnisapV2Contract(address common.Address, swapVersion string, quoterAddress string) *UniswapV2Contract {
	return &UniswapV2Contract{
		baseContract: baseContract{
			Address: address,
			Abi:     uniswapEthAbi, // 默认使用 eth abi
		},
		swapVersion:  swapVersion,
		quoteAbi:     quoterAbi,
		quoteAddress: common.HexToAddress(quoterAddress),
	}
}

func (c *UniswapV2Contract) PackInput(methodName string, fromAmount, minAmount *big.Int, path []common.Address, to common.Address) (ethereum.CallMsg, error) {
	swapAbi := uniswapEthAbi
	if strings.Contains(methodName, "AVAX") {
		swapAbi = uniswapAvaxAbi
	}

	deadline := big.NewInt(time.Now().Unix() + 3600)

	if c.swapVersion == versionV3 {
		pathByte, err := encodePath(path)
		if err != nil {
			return ethereum.CallMsg{}, err
		}
		return packInput(uniswapV3Abi, common.Address{}, c.Address, methodExactInput, ExactInputParams{
			Path:             pathByte,
			Recipient:        to,
			Deadline:         deadline,
			AmountIn:         fromAmount,
			AmountOutMinimum: minAmount,
		})
	}

	if strings.HasPrefix(methodName, "swapExactTokens") {
		return packInput(swapAbi, common.Address{}, c.Address, methodName, fromAmount, minAmount, path, to, deadline)
	} else {
		return packInput(swapAbi, common.Address{}, c.Address, methodName, minAmount, path, to, deadline)
	}
}

// EncodePath encode path to bytes
func encodePath(path []common.Address) (encoded []byte, err error) {
	fees := make([]int, len(path)-1)
	for i := range fees {
		fees[i] = 3000
	}

	encoded = make([]byte, 0, len(fees)*Offset+AddrSize)
	for i := 0; i < len(fees); i++ {
		encoded = append(encoded, path[i].Bytes()...)
		feeBytes := big.NewInt(int64(fees[i])).Bytes()
		feeBytes = common.LeftPadBytes(feeBytes, 3)
		encoded = append(encoded, feeBytes...)
	}
	encoded = append(encoded, path[len(path)-1].Bytes()...)
	return
}

func (c *UniswapV2Contract) GetAmountsIn(client *ethclient.Client, amountOut *big.Int, path []common.Address) ([]*big.Int, error) {
	if c.swapVersion == versionV3 {
		return c.quoteExactOutput(client, amountOut, reverseAddress(path))
	}

	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodGetAmountIn, amountOut, path)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := make([]*big.Int, len(path)-1)
	err = unpackOutput(&resp, c.Abi, methodGetAmountIn, resData)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func reverseAddress(s []common.Address) []common.Address {
	a := make([]common.Address, len(s))
	copy(a, s)

	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}

	return a
}

func (c *UniswapV2Contract) quoteExactInput(client *ethclient.Client, amountIn *big.Int, path []common.Address) ([]*big.Int, error) {
	opts := &bind.CallOpts{}
	pathByte, err := encodePath(path)
	if err != nil {
		return nil, err
	}
	msg, err := packInput(c.quoteAbi, opts.From, c.quoteAddress, methodQuoteExactInput, pathByte, amountIn)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := big.NewInt(0)
	err = unpackOutput(&resp, c.quoteAbi, methodQuoteExactInput, resData)
	if err != nil {
		return nil, err
	}
	return []*big.Int{resp}, nil
}

func (c *UniswapV2Contract) quoteExactOutput(client *ethclient.Client, amountOut *big.Int, path []common.Address) ([]*big.Int, error) {
	opts := &bind.CallOpts{}
	pathByte, err := encodePath(path)
	if err != nil {
		return nil, err
	}
	msg, err := packInput(c.quoteAbi, opts.From, c.quoteAddress, methodQuoteExactOutput, pathByte, amountOut)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := big.NewInt(0)
	err = unpackOutput(&resp, c.quoteAbi, methodQuoteExactOutput, resData)
	if err != nil {
		return nil, err
	}
	return []*big.Int{resp}, nil
}

func (c *UniswapV2Contract) GetAmountsOut(client *ethclient.Client, amountIn *big.Int, path []common.Address) ([]*big.Int, error) {
	if c.swapVersion == versionV3 {
		return c.quoteExactInput(client, amountIn, path)
	}

	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodGetAmountsOut, amountIn, path)
	if err != nil {
		return nil, err
	}
	resData, err := bind.ContractCaller(client).CallContract(context.Background(), msg, opts.BlockNumber)
	if err != nil {
		return nil, err
	}
	resp := make([]*big.Int, len(path)-1)
	err = unpackOutput(&resp, c.Abi, methodGetAmountsOut, resData)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type Erc20Contract struct {
	baseContract
}

func newErc20Contract(address common.Address) *Erc20Contract {
	return &Erc20Contract{
		baseContract{
			Address: address,
			Abi:     erc20Abi,
		},
	}
}

func (c *Erc20Contract) Approve(rpc string, client *ethclient.Client, account *eth.Account, approveTo common.Address, amount *big.Int) (string, error) {
	ctx := context.Background()
	accountAddress := common.HexToAddress(account.Address())
	opts := &bind.TransactOpts{
		From: accountAddress,
	}
	// 向 so diamond 合约 approve erc20 token
	msg, err := packInput(c.Abi, opts.From, c.Address, methodApprove, approveTo, amount)
	if err != nil {
		return "", err
	}
	// 获取 gas gasprice,构造 tx, encode 成 []byte，使用 account 签名，使用 client 发送
	// 交易数据由服务端构造，sdk 组装成 tx, marshalBinary 之后交给 wallet-sdk 处理签名 + 发送
	rawBytes, err := createRawTx(ctx, client, accountAddress, &c.Address, msg, big.NewInt(0))
	if err != nil {
		return "", err
	}
	return signAndSendTx(rawBytes, rpc, account)
}

func signAndSendTx(rawBytes []byte, rpc string, account *eth.Account) (string, error) {
	wallet := eth.NewChainWithRpc(rpc)
	privateKeyHex, err := account.PrivateKeyHex()
	if err != nil {
		return "", err
	}
	tx, err := eth.NewTransactionFromHex(hex.EncodeToString(rawBytes))
	if err != nil {
		return "", err
	}
	signedTx, err := wallet.SignTransaction(privateKeyHex, tx)
	if err != nil {
		return "", err
	}
	sendTxResult, err := wallet.SendRawTransaction(signedTx.Value)
	if err != nil {
		return "", err
	}
	return sendTxResult, nil
}

func createRawTx(ctx context.Context,
	client *ethclient.Client,
	accountAddress common.Address,
	contract *common.Address,
	msg ethereum.CallMsg,
	value *big.Int) ([]byte, error) {
	// 获取 gas gasprice,构造 tx, encode 成 []byte，使用 account 签名，使用 client 发送
	// 交易数据由服务端构造，sdk 组装成 tx, marshalBinary 之后交给 wallet-sdk 处理签名 + 发送
	nonce, err := client.PendingNonceAt(ctx, accountAddress) // 由  signer 实现
	if err != nil {
		return nil, err
	}
	msg.Value = value
	estimateGas, err := client.EstimateGas(ctx, msg)
	if err != nil {
		return nil, err
	}
	gasLimit := uint64(float64(estimateGas) * 5)

	var (
		maxPriorityFee *big.Int
		maxFee         *big.Int
	)
	priorityRate := 1.5
	maxFeeRate := 1.1
	// base fee
	header, err := client.HeaderByNumber(ctx, big.NewInt(-1))
	if err != nil || header.BaseFee == nil {
		var gasPrice *big.Int
		gasPrice, err = client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, err
		}
		maxPriorityFee = decimal.NewFromBigInt(gasPrice, 0).Mul(decimal.NewFromFloat(maxFeeRate)).BigInt()
		maxFee = maxPriorityFee
	} else {
		// tip fee
		priorityFee, err := client.SuggestGasTipCap(ctx) // GasFeeCap maxFeePerGas
		if err != nil {
			return nil, err
		}

		// MaxPriorityFee = SuggestPriorityFee * priorityRate
		// MaxFee = (MaxPriorityFee + BaseFee) * maxFeeRate
		maxPriorityFee = decimal.NewFromBigInt(priorityFee, 0).Mul(decimal.NewFromFloat(priorityRate)).BigInt()
		maxFee = decimal.NewFromBigInt(big.NewInt(0).Add(maxPriorityFee, header.BaseFee), 0).Mul(decimal.NewFromFloat(maxFeeRate)).BigInt()
	}

	if err != nil {
		errInfo := err.Error()
		// 尽管用户的余额不够，也需要返回可兑换的结果，用户最后执行的时候会再 estimateGas，那时候会失败
		if strings.Contains(errInfo, "insufficient funds for transfer") {
			err = nil
		} else {
			return nil, err
		}
	}

	rawTx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		To:        contract,
		Value:     value,
		Gas:       gasLimit,
		GasFeeCap: maxFee,
		GasTipCap: maxPriorityFee,
		Data:      msg.Data,
	})
	return rawTx.MarshalBinary()
}

func packInput(pabi *abi.ABI, from, toContract common.Address, methodName string, args ...interface{}) (ethereum.CallMsg, error) {
	inputParams, err := pabi.Pack(methodName, args...)
	if err != nil {
		return ethereum.CallMsg{}, err
	}
	payload := strings.TrimPrefix(hex.EncodeToString(inputParams), "0x")
	payloadBuf, err := hex.DecodeString(payload)
	if err != nil {
		return ethereum.CallMsg{}, err
	}
	return ethereum.CallMsg{From: from, To: &toContract, Data: payloadBuf}, nil
}

func unpackOutput(out interface{}, pabi *abi.ABI, methodName string, resData []byte) error {
	method, ok := pabi.Methods[methodName]
	if !ok {
		return errors.New("not found method:" + methodName)
	}
	a, err := method.Outputs.Unpack(resData)
	if err != nil {
		return err
	}
	err = method.Outputs.Copy(out, a)
	if err != nil {
		return err
	}
	return nil
}
