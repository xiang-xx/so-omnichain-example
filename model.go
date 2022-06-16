package main

import (
	"encoding/hex"
	"math/big"
	"math/rand"
	"time"
)

type SoData struct {
	TransactionId      string
	Receiver           string
	SourceChainId      int
	SendingAssetId     string
	DestinationChainId int
	ReceivingAssetId   string
	Amount             *big.Int
}

func newSoData(receiver string, sourceChainId int, sendingAssetId string, destinationChainId int, receivingAssetId string, amount *big.Int) SoData {
	return SoData{
		TransactionId:      randomTransactionId(),
		Receiver:           receiver,
		SourceChainId:      sourceChainId,
		SendingAssetId:     sendingAssetId,
		DestinationChainId: destinationChainId,
		ReceivingAssetId:   receivingAssetId,
		Amount:             amount,
	}
}

func randomTransactionId() string {
	b := make([]byte, 32)
	rand.Seed(time.Now().Unix())
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
