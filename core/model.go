package core

import (
	"encoding/hex"
	"fmt"
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

func (d *SoData) print() {
	fmt.Println("===========================================================")
	fmt.Println("so data:")
	fmt.Printf("transactionId:      %s\n", hex.EncodeToString(d.TransactionId[:]))
	fmt.Printf("Receiver:           %s\n", d.Receiver.Hex())
	fmt.Printf("SourceChainId:      %s\n", d.SourceChainId.String())
	fmt.Printf("SendingAssetId:     %s\n", d.SendingAssetId.String())
	fmt.Printf("DestinationChainId: %s\n", d.DestinationChainId.String())
	fmt.Printf("ReceivingAssetId:   %s\n", d.ReceivingAssetId.String())
	fmt.Printf("Amount:             %s\n", d.Amount.String())
}

// 1. 通用Uniswap/PancakeSwap数据结构
// 2. 代表用fromAmount数量的sendingAssetId换取receivingAssetId
// 3. 从数据流图来看，用SwapData来表示在source swap上从ETH换USDC;
type SwapData struct {
	CallTo           common.Address
	ApproveTo        common.Address
	SendingAssetId   common.Address // eth 是传 0 地址，不管是 v2 还是 v3
	ReceivingAssetId common.Address // token address, eth 是 0 地址, v3 swap 是 weth，不能是 0 地址
	FromAmount       *big.Int       // swap start token amount
	CallData         []byte         //  The swap callData callData = abi.encodeWithSignature("swapExactETHForTokens", minAmount, [sendingAssetId, receivingAssetId], 以太坊SoDiamond地址, deadline)
}

func (d *SwapData) print() {
	fmt.Println("===========================================================")
	fmt.Println("swap data:")
	fmt.Printf("CallTo:              %s\n", d.CallTo.String())
	fmt.Printf("ApproveTo:           %s\n", d.ApproveTo.String())
	fmt.Printf("SendingAssetId:      %s\n", d.SendingAssetId.String())
	fmt.Printf("ReceivingAssetId:    %s\n", d.ReceivingAssetId.String())
	fmt.Printf("FromAmount:          %s\n", d.FromAmount.String())
	fmt.Printf("CallData:            %s\n", hex.EncodeToString(d.CallData))
}

// StargateData 传给 stargate 的数据
type StargateData struct {
	SrcStargatePoolId  *big.Int       // stargate 源 pool id
	DstStargateChainId uint16         // stargete 目的链 chain id，非 evmchainid，是 stargate 自己定义的 id
	DstStargatePoolId  *big.Int       // stargate 目标链 pool id
	MinAmount          *big.Int       // 目标链最小得到数量
	DstGasForSgReceive *big.Int       // 目的链 sgReceive 消耗的 gas,通过 sgReceiveForGas 预估
	DstSoDiamond       common.Address // 目的链 SoDiamond 地址
}

func (d *StargateData) print() {
	fmt.Println("===========================================================")
	fmt.Println("StargateData:")
	fmt.Printf("SrcStargatePoolId:      %s\n", d.SrcStargatePoolId)
	fmt.Printf("DstStargateChainId:     %d\n", d.DstStargateChainId)
	fmt.Printf("DstStargatePoolId:      %s\n", d.DstStargatePoolId)
	fmt.Printf("MinAmount:              %s\n", d.MinAmount)
	fmt.Printf("DstGasForSgReceive:     %s\n", d.DstGasForSgReceive)
	fmt.Printf("DstSoDiamond:           %s\n", d.DstSoDiamond)
}

func newStargateData(fromChain Chain, toChain Chain, minAmount, dstGas *big.Int) StargateData {
	srcPoolId := big.NewInt(int64(fromChain.StargetaPoolId))
	dstPoolId := big.NewInt(int64(toChain.StargetaPoolId))
	data := StargateData{}
	data.SrcStargatePoolId = srcPoolId
	data.DstStargateChainId = uint16(toChain.StargateChainId)
	data.DstStargatePoolId = dstPoolId
	data.MinAmount = minAmount
	data.DstGasForSgReceive = dstGas
	data.DstSoDiamond = common.HexToAddress(toChain.SoDiamond)
	return data
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

// newSwapData 构造 SwapData
func newSwapData(chain Chain, fromTokenAddress string, toTokenAddress string, fromAmount, minAmount *big.Int) (SwapData, []common.Address, error) {
	ethName := "ETH"
	if chain.Name == "avax-test" {
		ethName = "AVAX"
	}

	// swap 合约
	swapVersion := versionV2
	quoteAdderss := ""
	if chain.Swap[0][1] == "ISwapRouter" {
		swapVersion = versionV3
		quoteAdderss = chain.Swap[0][2]
	}

	// swap method & path
	funcName := getSwapFuncName(fromTokenAddress, toTokenAddress, ethName)
	swapContractAddress := common.HexToAddress(chain.Swap[0][0])
	path := make([]common.Address, 0)
	if isZeroAddress(fromTokenAddress) {
		path = append(path, common.HexToAddress(chain.Weth))
	} else {
		path = append(path, common.HexToAddress(fromTokenAddress))
	}
	if isZeroAddress(toTokenAddress) {
		path = append(path, common.HexToAddress(chain.Weth))
	} else {
		path = append(path, common.HexToAddress(toTokenAddress))
	}

	callMsg, err := newUnisapV2Contract(swapContractAddress, swapVersion, quoteAdderss).
		PackInput(funcName, fromAmount, minAmount, path, common.HexToAddress(chain.SoDiamond))
	if err != nil {
		return SwapData{}, path, err
	}

	// v3 swap receiveAssetId 是 weth，不能是 0 地址
	if isZeroAddress(toTokenAddress) && swapVersion == versionV3 {
		toTokenAddress = chain.Weth
	}

	return SwapData{
		CallTo:           swapContractAddress,
		ApproveTo:        swapContractAddress,
		SendingAssetId:   common.HexToAddress(fromTokenAddress),
		ReceivingAssetId: common.HexToAddress(toTokenAddress), // token address, eth 是 0 地址, v3 swap 是 weth，不能是 0 地址
		FromAmount:       fromAmount,
		CallData:         callMsg.Data,
	}, path, nil
}

func getSwapFuncName(fromTokenAddress, toTokenAddress string, ethName string) string {
	fromName := "Tokens"
	toName := fromName
	if isZeroAddress(fromTokenAddress) {
		fromName = ethName
	}
	if isZeroAddress(toTokenAddress) {
		toName = ethName
	}
	return fmt.Sprintf("swapExact%sFor%s", fromName, toName)
}

func isZeroAddress(address string) bool {
	return address == zeroAddress || address == zeroAddressNoPrefix
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
