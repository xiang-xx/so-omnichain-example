# so-omnichain-example
SoOmnichain go example

仿照 [SoOmnichain python swap](https://github.com/chainx-org/SoOmnichain/blob/main/scripts/swap.py) 写的一个 Go 样例。

运行方式:
```sh
export words="xxx xxx xxx"  # 安全起见最好还是别这么执行
go run main.go -fc rinkeby -tc avax-test -ft usdc -tc eth
```

vscode config 运行示例:
```json
{
    "name": "rinkeby(usdc)->rinkeby(eth)",
    "type": "go",
    "request": "launch",
    "mode": "auto",
    "program": "${workspaceFolder}",
    "args": ["-fc", "rinkeby", "-tc", "rinkeby", "-ft", "usdc", "-tt", "eth"],
    "env": {
        "words": "xxx xxx"
    }
},
{
    "name": "rinkeby(eth)->rinkeby(usdc)",
    "type": "go",
    "request": "launch",
    "mode": "auto",
    "program": "${workspaceFolder}",
    "args": ["-fc", "rinkeby", "-tc", "rinkeby", "-ft", "eth", "-tt", "usdc"],
    "env": {
        "words": "xxx xxx"
    }
},
```
