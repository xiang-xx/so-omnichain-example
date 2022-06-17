package main

import (
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// SoData 代表单笔 swap 数据，用于链路 swap 追踪
type SoData struct {
	TransactionId      [32]byte // 唯一交易ID， 32字节
	Receiver           common.Address
	SourceChainId      *big.Int
	SendingAssetId     common.Address
	DestinationChainId *big.Int
	ReceivingAssetId   common.Address
	Amount             *big.Int
}

// 1. 通用Uniswap/PancakeSwap数据结构
// 2. 代表用fromAmount数量的sendingAssetId换取receivingAssetId
// 3. 从数据流图来看，用SwapData来表示在source swap上从ETH换USDC;
type SwapData struct {
	CallTo           common.Address
	ApproveTo        common.Address
	SendingAssetId   common.Address
	ReceivingAssetId common.Address // token address, eth 是 0 地址
	FromAmount       *big.Int       // swap start token amount
	CallData         []byte         //  The swap callData callData = abi.encodeWithSignature("swapExactETHForTokens", minAmount, [sendingAssetId, receivingAssetId], 以太坊SoDiamond地址, deadline)
}

// StargeteData 传给 stargate 的数据
type StargeteData struct {
	SrcStargatePoolId  *big.Int       // stargate 源 pool id
	DstStargateChainId *big.Int       // stargete 目的链 chain id，非 evmchainid，是 stargate 自己定义的 id
	DstStargatePoolId  *big.Int       // stargate 目标链 pool id
	MinAmount          *big.Int       // 目标链最小得到数量
	DstGasForSgReceive *big.Int       // 目的链 sgReceive 消耗的 gas,通过 sgReceiveForGas 预估
	DstSoDiamond       common.Address // 目的链 SoDiamond 地址
}

func newSoData(receiver string, sourceChainId int, sendingAssetId string, destinationChainId int, receivingAssetId string, amount *big.Int) SoData {
	return SoData{
		TransactionId:      randomTransactionId(),
		Receiver:           common.HexToAddress(receiver),
		SourceChainId:      big.NewInt(int64(sourceChainId)),
		SendingAssetId:     common.HexToAddress(sendingAssetId),
		DestinationChainId: big.NewInt(int64(destinationChainId)),
		ReceivingAssetId:   common.HexToAddress(receivingAssetId),
		Amount:             amount,
	}
}

func randomTransactionId() [32]byte {
	var res [32]byte
	rand.Seed(time.Now().Unix())
	_, err := rand.Read(res[:])
	if err != nil {
		panic(err)
	}
	return res
}
