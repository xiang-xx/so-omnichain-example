package main

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"github.com/coming-chat/wallet-SDK/core/eth"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	methodApprove = "approve"
)

type baseContract struct {
	Address common.Address
	ChainId *big.Int
	Abi     *abi.ABI
}

type DiamondContract struct {
	baseContract
}

func newDiamondContract(address common.Address, abi *abi.ABI) *DiamondContract {
	return &DiamondContract{
		baseContract{
			Address: address,
			Abi:     abi,
		},
	}
}

func (c *DiamondContract) SgReceiveForGas(client *ethclient.Client, soData SoData, stargatePoolId *big.Int, toChainSwapData []SwapData) (uint64, error) {
	opts := &bind.CallOpts{}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodSgReceiveForGas, soData, stargatePoolId, toChainSwapData)
	if err != nil {
		return 0, err
	}
	return bind.ContractTransactor(client).EstimateGas(context.Background(), msg)

}

type Erc20Contract struct {
	baseContract
}

func newErc20Contract(address common.Address, abi *abi.ABI) *Erc20Contract {
	return &Erc20Contract{
		baseContract{
			Address: address,
			Abi:     abi,
		},
	}
}

func (c *Erc20Contract) Approve(rpc string, client *ethclient.Client, account *eth.Account, amount *big.Int) (string, error) {
	ctx := context.Background()
	accountAddress := common.HexToAddress(account.Address())
	opts := &bind.TransactOpts{
		From: accountAddress,
	}
	msg, err := packInput(c.Abi, opts.From, c.Address, methodApprove, accountAddress, amount)
	if err != nil {
		return "", err
	}
	// 获取 gas gasprice,构造 tx, encode 成 []byte，使用 account 签名，使用 client 发送
	nonce, err := client.PendingNonceAt(ctx, accountAddress) // 由  signer 实现
	if err != nil {
		return "", err
	}
	estimateGas, err := client.EstimateGas(ctx, msg)
	if err != nil {
		return "", err
	}
	gasLimit := uint64(float64(estimateGas) * 1.3)
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", err
	}

	rawTx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &c.Address,
		Value:    big.NewInt(0),
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     msg.Data,
	})
	rawBytes, err := rawTx.MarshalBinary()
	if err != nil {
		return "", err
	}
	decodeTx := types.NewTx(&types.LegacyTx{})
	err = decodeTx.UnmarshalBinary(rawBytes)
	if err != nil {
		return "", err
	}
	wallet := eth.NewEthChain()
	wallet.CreateRemote(rpc)
	privateKeyHex, err := account.PrivateKeyHex()
	if err != nil {
		return "", err
	}
	signedTx, err := wallet.SignTransaction(decodeTx, privateKeyHex)
	if err != nil {
		return "", err
	}
	sendTxResult, err := wallet.SendRawTransaction(signedTx.TxHex)
	if err != nil {
		return "", err
	}
	return sendTxResult, nil
}

func packInput(pabi *abi.ABI, from, to common.Address, methodName string, args ...interface{}) (ethereum.CallMsg, error) {
	inputParams, err := pabi.Pack(methodName, args...)
	if err != nil {
		return ethereum.CallMsg{}, err
	}
	payload := strings.TrimPrefix(hex.EncodeToString(inputParams), "0x")
	payloadBuf, err := hex.DecodeString(payload)
	if err != nil {
		return ethereum.CallMsg{}, err
	}
	return ethereum.CallMsg{From: from, To: &to, Data: payloadBuf}, nil
}

func unpackOutput(out interface{}, pabi *abi.ABI, methodName string, paramsStr string) error {
	method, ok := pabi.Methods[methodName]
	if !ok {
		return errors.New("not found method:" + methodName)
	}
	paramsStr = strings.TrimPrefix(paramsStr, "0x")
	data, err := hex.DecodeString(paramsStr)
	if err != nil {
		return err
	}
	a, err := method.Outputs.Unpack(data)
	if err != nil {
		return err
	}
	err = method.Outputs.Copy(out, a)
	if err != nil {
		return err
	}
	return nil
}
