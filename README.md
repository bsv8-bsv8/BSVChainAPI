# BSVChainAPI

面向业务最小语义的 BSV 链 API 模块。

第一阶段只保留 4 个能力：

- `GetUTXOs`
- `GetTipHeight`
- `Broadcast`
- `GetTxDetail`

设计约束：

- 库是主形态，端口只是附加运行模式。
- 每次调用显式传 `provider + network + profile`。
- 认证差异不进入业务 API，通过 `profile` 对应的路由配置解决。
- HTTP 端口协议是独立 JSON，不镜像上游 Whatsonchain / Bitails。
- 当前已接入真实 provider：`whatsonchain`、`bitails`、`gorillapool_arc`、`taal_arc`、`taal_legacy`。

最小配置示例：

```json
{
  "listen": "127.0.0.1:18222",
  "routes": [
    {
      "provider": "whatsonchain",
      "network": "test",
      "profile": "default",
      "protect": {
        "min_interval": "1s"
      }
    },
    {
      "provider": "bitails",
      "network": "test",
      "profile": "default",
      "protect": {
        "min_interval": "100ms"
      }
    },
    {
      "provider": "whatsonchain",
      "network": "main",
      "profile": "paid-a",
      "auth": {
        "mode": "header",
        "name": "X-API-Key",
        "value": "your-key"
      },
      "protect": {
        "min_interval": "200ms"
      }
    },
    {
      "provider": "bitails",
      "network": "main",
      "profile": "paid-a",
      "auth": {
        "value": "your-bitails-api-key"
      },
      "protect": {
        "min_interval": "100ms"
      }
    }
  ]
}
```

启动：

```bash
go run ./cmd/bsv-chainapi -config runtime.json
```

真实公网对比测试：

```bash
BSV_CHAINAPI_LIVE=1 /home/david/.gvm/gos/go1.26.0/bin/go test -run Live ./...
```

约束：

- 只测试 `GetUTXOs`、`GetTipHeight`、`GetTxDetail`
- 不做真实 `Broadcast` 测试

补充：

- `Broadcast` 的多路接盘策略由 `TxSubmitRouter` 负责，route 本身只负责单个上游适配。
- `GetRouteInfoContext` 用于启动期校验 route 是否真实存在、以及是否支持提交能力。
