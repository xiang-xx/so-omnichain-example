package core

type Config struct {
	Networks Networks `yaml:"networks"`
}

type Networks struct {
	Rinkeby      Chain `yaml:"rinkeby"`
	PolygonTest  Chain `yaml:"polygon-test"`
	AvaxTest     Chain `yaml:"avax-test"`
	OptimismTest Chain `yaml:"optimism-test"`
}

type Chain struct {
	Name            string     `yaml:"name"`
	ChainId         int        `yaml:"chainid"`
	Rpc             string     `yaml:"rpc"`
	StargateRouter  string     `yaml:"stargate_router"`
	SoDiamond       string     `yaml:"so_diamond"`
	StargateChainId int        `yaml:"stargate_chainid"`
	StargetaPoolId  int        `yaml:"stargate_poolid"`
	Usdc            string     `yaml:"usdc"`
	Weth            string     `yaml:"weth"`
	Swap            [][]string `yaml:"swap"`
}
