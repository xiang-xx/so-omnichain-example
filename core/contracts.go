package core

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"strconv"
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
	methodGetAmountsOut               = "getAmountsOut"
	methodGetAmountIn                 = "getAmountsIn"
	methodEstimateStargateFinalAmount = "estimateStargateFinalAmount"
	methodGetSoFee                    = "getSoFee"
	methodGetAmountBeforeSoFee        = "getAmountBeforeSoFee"
)

var (
	diamondAbi     *abi.ABI
	erc20Abi       *abi.ABI
	uniswapEthAbi  *abi.ABI
	uniswapAvaxAbi *abi.ABI
)

func init() {
	initAbi(&diamondAbi, "abi/so_diamond.json")
	initAbi(&erc20Abi, "abi/erc20.json")
	initAbi(&uniswapEthAbi, "abi/IUniswapV2Router02.json")
	initAbi(&uniswapAvaxAbi, "abi/IUniswapV2Router02AVAX.json")
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
}

func newUnisapV2Contract(address common.Address) *UniswapV2Contract {
	return &UniswapV2Contract{
		baseContract{
			Address: address,
			Abi:     uniswapEthAbi, // 默认使用 eth abi
		},
	}
}

func (c *UniswapV2Contract) PackInput(methodName string, fromAmount, minAmount *big.Int, path []common.Address, to common.Address) (ethereum.CallMsg, error) {
	swapAbi := uniswapEthAbi
	if strings.Contains(methodName, "AVAX") {
		swapAbi = uniswapAvaxAbi
	}

	deadline := big.NewInt(time.Now().Unix() + 3600)
	if strings.HasPrefix(methodName, "swapExactTokens") {
		return packInput(swapAbi, common.Address{}, c.Address, methodName, fromAmount, minAmount, path, to, deadline)
	} else {
		return packInput(swapAbi, common.Address{}, c.Address, methodName, minAmount, path, to, deadline)
	}
}

func (c *UniswapV2Contract) GetAmountsIn(client *ethclient.Client, amountOut *big.Int, path []common.Address) ([]*big.Int, error) {
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

func (c *UniswapV2Contract) GetAmountsOut(client *ethclient.Client, amountIn *big.Int, path []common.Address) ([]*big.Int, error) {
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
	decodeTx := types.NewTx(&types.DynamicFeeTx{})
	err := decodeTx.UnmarshalBinary(rawBytes)
	if err != nil {
		return "", err
	}
	wallet := eth.NewChainWithRpc(rpc)
	privateKeyHex, err := account.PrivateKeyHex()
	if err != nil {
		return "", err
	}
	tx := eth.NewTransaction(strconv.Itoa(int(decodeTx.Nonce())),
		decodeTx.GasTipCap().String(),
		strconv.Itoa(int(decodeTx.Gas())),
		decodeTx.To().String(), decodeTx.Value().String(), hex.EncodeToString(decodeTx.Data()))
	tx.MaxPriorityFeePerGas = decodeTx.GasPrice().Text(10)
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
	gasLimit := uint64(float64(estimateGas) * 1.1)

	priorityFee, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, err
	}

	rawTx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		To:        contract,
		Value:     value,
		Gas:       gasLimit,
		GasFeeCap: priorityFee,
		GasTipCap: decimal.NewFromBigInt(priorityFee, 0).Mul(decimal.NewFromFloat(1.2)).BigInt(),
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
