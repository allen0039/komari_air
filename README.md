# komari_air

基于 [Komari](https://github.com/komari-monitor/komari) 的社区二次开发版本，专注于自托管服务器监控。项目保留节点监控和常用管理能力，精简不需要的入口，并增加回程路由探测与 Agent 升级管理。

> 本项目不是 Komari 官方发行版，也不隶属于原项目维护者。

## 功能概览

- 查看节点在线状态、资源指标和历史数据；支持 Ping、远程命令、主题与本地插件管理。
- 支持密码登录、会话管理和可选的两步验证。
- 支持 Linux 节点的三网回程探测，可查看历史采样和不确定结果；探测结论不代表带宽或线路质量保证。详见 [功能说明](docs/return-route-beta.md)。
- 提供 Agent 版本检查、下载和分批升级能力；Agent 由独立仓库 [komari_agent](https://github.com/allen0039/komari_agent) 维护。
- 对仪表盘的数据加载做了优化，减少不必要的重复请求。

本分支移除了 Web 终端、文件管理与传输、OAuth/SSO/第三方登录、管理员 API Key 登录和插件市场。本地插件安装与管理仍然保留。

## 快速部署

需要一台可以运行 Docker 和 Docker Compose 的 Linux 主机。默认对外端口为 `25774`，数据保存在部署目录的 `data/` 中。

```bash
git clone https://github.com/allen0039/komari_air.git
cd komari_air
docker compose up -d
```

访问 `http://<服务器地址>:25774` 完成初始化。公开部署时，请配置 HTTPS 反向代理、强密码，并按需限制管理入口的访问范围。

也可以使用仓库提供的 [部署脚本](scripts/deploy.sh)。该脚本会安装缺失的依赖、拉取预构建镜像并等待健康检查；运行前请先阅读脚本：

```bash
curl -fsSLO https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/deploy.sh
sudo bash deploy.sh
```

脚本默认安装到 `/opt/komari_air`，可通过 `KOMARI_INSTALL_DIR`、`KOMARI_PORT` 和 `KOMARI_TZ` 调整目录、端口与时区。再次运行部署命令可更新服务。镜像发布在 `docker.io/allen0039/komari_air` 和 `ghcr.io/allen0039/komari_air`。

常用管理命令（使用部署脚本安装后）：

```bash
cd /opt/komari_air
sudo ./scripts/deploy.sh status
sudo ./scripts/deploy.sh logs
sudo ./scripts/deploy.sh restart
sudo ./scripts/deploy.sh stop
```

## 从源码构建

需要 Node.js 22+、npm、Go 1.25+、`tar`、`zstd`，以及 SQLite 构建所需的 CGO 工具链。

```bash
./scripts/build.sh
```

脚本安装前端依赖、构建并打包前端资源，然后编译服务端到 `build/komari`。单独开发前端可运行：

```bash
cd web
cp .env.example .env.development
npm ci
npm run dev
```

主控版本见 `VERSION`，镜像内置的稳定版 Agent 版本见 `AGENT_VERSION`。发布主控版本不会自动更改 Agent 版本。

## 项目结构

```text
server/   服务端
web/      管理后台与公开监控页面
scripts/  构建、部署和发布脚本
docs/     功能说明
```

## 安全与数据

本项目具备远程命令等管理能力，只应部署在你有权管理的系统上。不要将数据库、环境文件、Agent Token 或其他凭据提交到仓库。部署数据位于 `data/`，升级或停止容器前建议自行备份。

## 上游与许可

项目最初基于以下上游源码整理：

- Server：[`komari-monitor/komari@0ca87aa`](https://github.com/komari-monitor/komari/commit/0ca87aafd184ed75f9030ede0902772142af5eec)
- Web：[`komari-monitor/komari-web@3324844`](https://github.com/komari-monitor/komari-web/commit/3324844cfa347f18c83435f1ccf5634df7e5b768)
- Agent：[`komari-monitor/komari-agent@c7bafb7`](https://github.com/komari-monitor/komari-agent/commit/c7bafb79b1a73ed9e14e2c248ae4d4163e506036)

`server/` 保留上游 MIT [许可证](server/LICENSE)和 [NOTICE](server/NOTICE)。Agent 的许可证见独立仓库。导入时的上游 Web 源码未附许可证文件；本仓库不替其推定授权，也没有声明覆盖全部目录的统一许可证。
