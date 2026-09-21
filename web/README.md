# komari_air web

这是 `komari_air` 的 React/Vite 前端，已移除插件市场、文件管理、Web 终端、OAuth 和 SSO 界面。

## 开发

```bash
cp .env.example .env.development
npm ci
npm run dev
```

## 构建

```bash
npm ci
npm run build
```

生产构建会输出到 `dist/`。从仓库根目录运行 `./scripts/build.sh` 可继续将它打包进服务端。

上游 `komari-monitor/komari-web` 在本项目导入的版本中没有提供许可证文件。请阅读仓库根目录 README 中的许可证说明。

