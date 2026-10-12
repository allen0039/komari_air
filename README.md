# komari_air

基于 [Komari](https://github.com/komari-monitor/komari) 的社区二次开发版本，面向自托管服务器监控。项目保留节点监控、延迟监测和常用管理能力，提供预构建容器镜像与面板内的 Agent 升级管理。

> 本项目不是 Komari 官方发行版，也不隶属于原项目维护者。

## 功能

- **节点监控**：查看在线状态、资源指标和历史数据，包括 CPU、内存、磁盘、网络及受支持设备的 GPU 信息。
- **延迟监测**：创建 Ping 任务，在公开页面和管理仪表盘查看延迟、丢包及统计结果。
- **通知**：配置离线、负载和消息发送渠道等通知规则。
- **Agent 管理**：面板提供安装脚本和匹配平台的 Agent 下载；支持先升级选定节点，再分批升级在线节点，并查看逐节点结果。离线或不支持远程升级的 Agent 需要在节点侧处理。Agent 源码在独立仓库 [komari_agent](https://github.com/allen0039/komari_agent) 中维护。
- **主题与插件**：内置 [Emerald Globe Pro](https://github.com/allen0039/komari-theme-emerald-globe-pro) 作为新安装的默认公开页面主题；保留原版 `default` 主题、主题市场，以及通过 ZIP 上传的本地插件管理。
- **账号与运维**：密码登录、会话管理、可选双因素认证、数据库迁移/恢复、备份和 GeoIP 设置。

### 有意移除的功能

当前分支不提供以下入口：

- Web 终端、文件管理和文件传输
- OAuth、SSO 和其他第三方管理员登录
- 管理员 API Key 登录
- 插件市场（本地插件管理仍然保留；主题市场仍然可用）
- 三网回程路由探测

Agent 自动发现使用独立的注册密钥，不等同于管理员登录方式。

已有部署的主题设置会保留。可在后台「主题管理」中切换到 `emerald-globe-pro`，或切回原版 `default`。新安装默认使用 Emerald Globe Pro；管理后台仍使用内置的原版界面。

## 快速部署

需要一台运行 Docker 和 Docker Compose 的 Linux 主机。默认端口为 `25774`，数据持久化在部署目录的 `data/` 中。

### Docker Compose

```bash
git clone https://github.com/allen0039/komari_air.git
cd komari_air
docker compose pull
docker compose up -d
```

访问 `http://<服务器地址>:25774` 完成初始化。公开部署时，请在前面配置 HTTPS 反向代理、设置强密码，并限制管理入口的访问范围。

### 一键部署脚本

脚本会检查并安装缺失的 Git、Docker 和 Docker Compose，拉取预构建镜像，启动容器并等待健康检查通过。重复运行可更新现有部署；脚本执行前请先阅读其内容。

```bash
curl -fsSLO https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/deploy.sh
sudo bash deploy.sh
```

默认安装目录为 `/opt/komari_air`。可通过环境变量调整安装目录、端口和时区：

```bash
curl -fsSL https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/deploy.sh \
  | sudo env KOMARI_INSTALL_DIR=/srv/komari_air KOMARI_PORT=8080 KOMARI_TZ=Asia/Shanghai bash
```

镜像发布在 Docker Hub 和 GHCR：

- `docker.io/allen0039/komari_air`
- `ghcr.io/allen0039/komari_air`

使用部署脚本安装后，可在安装目录执行：

```bash
cd /opt/komari_air
sudo ./scripts/deploy.sh status
sudo ./scripts/deploy.sh logs
sudo ./scripts/deploy.sh restart
sudo ./scripts/deploy.sh stop
```

### Agent 安装

容器启动后，在管理后台添加节点并复制生成的安装命令。面板会根据节点平台提供对应脚本和 Agent 构建；也可以直接访问以下公开端点：

- `GET /api/public/agent/install.sh`
- `GET /api/public/agent/install.ps1`
- `GET /api/public/agent/version`
- `GET /api/public/agent/download/:os/:arch`

Agent 与主控独立版本化。`VERSION` 表示主控版本，`AGENT_VERSION` 表示镜像内置的稳定版 Agent。面板升级不会直接替换已有节点的 Agent；请在后台确认节点版本，并按需执行升级。

### Agent 卸载

在**安装 Agent 的 Linux 节点**上运行 [卸载脚本](scripts/uninstall-agent.sh)。脚本支持 systemd 系统服务、systemd 用户服务、OpenRC、OpenWrt/procd 和 Upstart；默认服务名为 `komari-agent`。安装目录优先从该服务的配置读取，系统服务未找到时使用 `/opt/komari`。

一键下载并卸载：

```bash
curl -fsSL https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/uninstall-agent.sh \
  | sudo sh
```

也可以先下载脚本，预览清理内容后再执行：

```bash
curl -fsSLo uninstall-agent.sh https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/uninstall-agent.sh
sudo sh uninstall-agent.sh --dry-run
sudo sh uninstall-agent.sh
```

脚本会停止服务、取消开机启动、移除服务定义，并清理 Agent 程序、`auto-discovery.json`、`managed-config.json`、`net_static.json`、相应临时文件和备份，以及 `agent.before-*` 等升级回滚副本。默认安装的已知 Agent 文件日志也会清理。目录中的主控程序、数据库和其他文件会保留；安装目录为空时才删除目录。遇到服务停止失败、程序仍在运行或目录不匹配等情况，脚本会中止并给出原因。

安装时自定义过服务名或目录的，使用对应参数；服务名需明确指定，脚本只处理选中的服务。目录已能从服务识别时可以省略 `--install-dir`：

```bash
sudo sh uninstall-agent.sh --service-name my-agent --install-dir /srv/komari
```

通过普通用户安装的 systemd 用户服务，请以**原安装用户**运行，不加 `sudo`。默认目录为 `${XDG_DATA_HOME:-$HOME/.local/share}/komari`：

```bash
sh uninstall-agent.sh --user --dry-run
sh uninstall-agent.sh --user
```

自行指定的外置 JSON 配置需要明确提供文件路径；外置受管配置对应 Agent 的 `AGENT_MANAGED_CONFIG` 环境变量。这些文件及其 `.tmp`、`.bak` 副本会一起清理，系统目录路径仍受脚本保护：

```bash
sudo sh uninstall-agent.sh \
  --config-file /srv/komari-config/config.json \
  --managed-config /srv/komari-config/managed-config.json
```

`--install-service-name` 是 `--service-name` 的兼容别名；运行 `sh uninstall-agent.sh --help` 查看全部选项。卸载会删除该节点本地保存的自动发现身份和流量统计，面板上的节点记录需在后台另行删除。系统 journal 历史日志会保留。

此脚本不处理 Docker Agent、Windows、macOS 或 NixOS。Docker Agent 需移除对应容器并清理其持久化身份文件；Windows、macOS 按各自的服务管理方式卸载；NixOS 需从声明式配置移除服务并重新构建系统。

## 从源码构建

需要 Node.js 22+、npm、Go 1.25+、`tar`、`zstd`，以及 SQLite 所需的 CGO 工具链。

```bash
./scripts/build.sh
```

构建脚本会安装前端依赖、构建 React/Vite 前端、压缩并嵌入原版前端资源，然后将服务端编译到 `build/komari`。Emerald Globe Pro 的打包文件位于 `server/web/public/bundledTheme/`，随服务端构建一同嵌入。单独开发前端：

```bash
cd web
cp .env.example .env.development
npm ci
npm run dev
```

常用检查命令：

```bash
cd server && go test ./...
cd ../web && npm run lint && npm run build
```

卸载脚本的模拟测试仅操作临时目录，用模拟服务命令验证清理范围和失败处理：

```bash
python3 scripts/tests/test_uninstall_agent.py
```

## 项目结构

```text
server/   Go 服务端、数据库和 Agent 协议
web/      React/Vite 管理后台与公开监控页面
scripts/  构建、部署和发布脚本
```

## 安全与数据

这是一个自托管的监控和运维面板。请只在你拥有或获准管理的系统中部署，并将管理入口放在 HTTPS 和受控网络之后。上传的插件可能声明执行子进程等权限，请在启用前检查来源和权限。

不要把数据库、`.env`、Agent Token、自动发现密钥或其他凭据提交到仓库。部署数据位于 `data/`；升级、迁移或停止容器前建议先备份。

## 上游与许可

项目最初基于以下上游源码整理：

- Server：[`komari-monitor/komari@0ca87aa`](https://github.com/komari-monitor/komari/commit/0ca87aafd184ed75f9030ede0902772142af5eec)
- Web：[`komari-monitor/komari-web@3324844`](https://github.com/komari-monitor/komari-web/commit/3324844cfa347f18c83435f1ccf5634df7e5b768)
- Agent：[`komari-monitor/komari-agent@c7bafb7`](https://github.com/komari-monitor/komari-agent/commit/c7bafb79b1a73ed9e14e2c248ae4d4163e506036)

`server/` 保留上游 MIT [许可证](server/LICENSE)和 [NOTICE](server/NOTICE)。Agent 许可证见独立仓库。导入时的上游 Web 源码未附许可证文件；本仓库不替其推定授权，也没有声明覆盖全部目录的统一许可证。
