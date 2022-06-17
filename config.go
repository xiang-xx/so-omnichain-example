package main

type Config struct {
	Networks Networks `yaml:"networks"`
}

type Networks struct {
	Rinkeby     Chain `yaml:"rinkeby"`
	PolygonTest Chain `yaml:"polygon-test"`
}

type Chain struct {
	ChainId         int        `yaml:"chainid"`
	Rpc             string     `yaml:"rpc"`
	StargateRouter  string     `yaml:"stargate_router"`
	SoDiamond       string     `yaml:"so_diamond"`
	StargateChainId string     `yaml:"stargate_chainid"`
	StargetaPoolId  string     `yaml:"stargate_poolid"`
	Usdc            string     `yaml:"usdc"`
	Weth            string     `yaml:"weth"`
	Swap            [][]string `yaml:"swap"`
}
