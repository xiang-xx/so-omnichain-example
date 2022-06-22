package main

import (
	"flag"
	"fmt"
	"so-omnichain-example/core"

	"github.com/fatih/color"
)

func main() {
	var (
		fromChain = flag.String("fc", "rinkeby", "from chain")
		toChain   = flag.String("tc", "polygon-test", "to chain")
		fromToken = flag.String("ft", "usdc", "from token")
		toToken   = flag.String("tt", "usdc", "to token")
	)
	flag.Parse()

	fmt.Println(color.HiBlueString("%s %s -->> %s %s", *fromChain, *fromToken, *toChain, *toToken))

	err := core.Swap(*fromChain, *toChain, *fromToken, *toToken)
	if err != nil {
		fmt.Println(color.HiRedString("Error: %s", err))
	}
}
