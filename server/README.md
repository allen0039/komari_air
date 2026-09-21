# komari_air server

这是 `komari_air` 的服务端组件，基于 `komari-monitor/komari` 精简。

服务端通过 `go:embed` 加载前端归档。请从仓库根目录运行：

```bash
./scripts/build.sh
```

如需单独构建服务端，请先按照 `web/public/readme.md` 生成 `web/public/defaultTheme/dist.tar.zst`，然后执行：

```bash
go build .
```

许可证和上游版权信息见 `LICENSE` 与 `NOTICE`。

