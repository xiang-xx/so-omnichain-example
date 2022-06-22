package core

import (
	"math/big"

	"github.com/shopspring/decimal"
)

// changeDecimals 修改 amount 精度，
// 比如之前精度是 6，修改为 18，则 输入 amount 1e6，输出 amount 1e18
func changeDecimals(amount *big.Int, fromDecimal, toDecimal int32) *big.Int {
	d := decimal.NewFromBigInt(amount, 0)
	d = d.Mul(decimal.NewFromBigInt(big.NewInt(1), toDecimal)).Div(decimal.NewFromBigInt(big.NewInt(1), fromDecimal))
	return d.BigInt()
}
