# komari_air

`komari_air` 是基于 Komari 的精简分支，目标是保留服务器监控的核心能力，减少不常用的管理功能、前端依赖和 Agent 攻击面。

本项目不是 Komari 官方发行版，也不隶属于原项目维护者。

## 精简内容

已移除：

- 插件市场（仍保留本地插件安装与管理）
- 文件管理和文件传输
- Web 终端
- OAuth、SSO 和第三方登录
- 管理员 API Key 登录

保留：

- 节点状态和历史数据监控
- Ping、批量远程命令等监控管理能力
- 密码登录、会话管理和可选的两步验证
- 主题管理
- 本地插件能力

## 目录结构

```text
server/  Komari 服务端
web/     管理后台和公开监控页面
scripts/ 本仓库构建脚本
```

节点 Agent 已拆分到独立仓库 [komari_agent](https://github.com/allen0039/komari_agent)。主控与 Agent 分别版本化；本仓库的 `AGENT_VERSION` 指定镜像内置的 Agent 版本，发布主控版本不会自动更新 Agent。

## 构建

建议使用 Node.js 22+、npm、Go 1.25+、`tar` 和 `zstd`。服务端使用 SQLite 时需要可用的 CGO 工具链。

一键构建：

```bash
./scripts/build.sh
```

脚本会执行以下步骤：

1. 使用 `npm ci` 安装前端依赖并构建前端。
2. 将前端产物压缩为服务端需要的嵌入资源。
3. 编译服务端到 `build/komari`。

Agent 请在独立仓库中构建。容器构建会按 `AGENT_VERSION` 从独立仓库的对应 Git tag 获取源码并编译，主控提供的 Agent 下载和一键升级仍使用该内置版本。

仅开发前端：

```bash
cd web
cp .env.example .env.development
npm ci
npm run dev
```

## 一键部署

适用于常见 Linux 服务器。脚本会自动安装缺失的 Git、Docker 和 Docker Compose，然后拉取 Docker Hub 上的预构建镜像并启动服务：

```bash
curl -fsSL https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/deploy.sh | sudo bash
```

已经是 `root` 用户时可以直接执行：

```bash
curl -fsSL https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/deploy.sh | bash
```

脚本支持 Debian、Ubuntu、RHEL/CentOS/Fedora、Alpine、Arch 和 openSUSE 等常见发行版。它会自动将部署配置仓库克隆到 `/opt/komari_air`，拉取 `docker.io/allen0039/komari_air:latest`，启动容器并等待健康检查通过。重复执行同一条命令即可更新。数据库保存在部署目录的 `data` 子目录中，更新和普通停止不会删除数据。

镜像也会同步发布到 `ghcr.io/allen0039/komari_air:latest`。GitHub Actions 发布工作流需要配置仓库 Secrets：`DOCKERHUB_USERNAME` 和 `DOCKERHUB_TOKEN`；其中 Docker Hub Token 建议使用仅有推送权限的 Access Token。

指定端口或时区：

```bash
curl -fsSL https://raw.githubusercontent.com/allen0039/komari_air/main/scripts/deploy.sh \
  | sudo env KOMARI_PORT=8080 KOMARI_TZ=Asia/Shanghai bash
```

常用管理命令：

```bash
cd /opt/komari_air
sudo ./scripts/deploy.sh status
sudo ./scripts/deploy.sh logs
sudo ./scripts/deploy.sh restart
sudo ./scripts/deploy.sh stop
```

首次启动后访问 `http://服务器IP:25774` 完成初始化。生产环境应在前面配置 HTTPS 反向代理，并限制管理入口访问范围。

## 安全说明

这是一个具有节点数据采集和远程命令能力的自托管工具。请只在你拥有或获准管理的系统中部署，使用 HTTPS，设置强密码，并限制管理后台的网络访问范围。

仓库不包含部署密钥、数据库、Agent Token、环境配置或构建后的二进制文件。请勿提交自己的 `.env`、数据库或 Agent 配置。

截至 2026-09-21，`govulncheck` 对服务端未发现可达漏洞；Agent 仍会报告 `GO-2026-5932`：旧自动更新依赖间接引用了已停止维护的 `golang.org/x/crypto/openpgp`，且上游没有提供修复版本。需要严格规避该依赖风险时，请使用 `--disable-auto-update` 并手动更新 Agent。前端依赖在首次发布前已执行 `npm audit fix`，`npm audit` 报告为 0 个漏洞。

## 上游来源

首次公开整理基于以下上游源码版本：

- Server: `komari-monitor/komari@0ca87aafd184ed75f9030ede0902772142af5eec`
- Web: `komari-monitor/komari-web@3324844cfa347f18c83435f1ccf5634df7e5b768`
- Agent: `komari-monitor/komari-agent@c7bafb79b1a73ed9e14e2c248ae4d4163e506036`

为避免携带上游提交历史、本地远程地址或历史中的敏感信息，本仓库使用全新的 Git 历史导入。

## 许可证

- `server/` 保留其上游 MIT 许可证和 NOTICE，见 `server/LICENSE` 与 `server/NOTICE`。
- Agent 的上游 MIT 许可证随独立仓库迁移，见 [komari_agent/LICENSE](https://github.com/allen0039/komari_agent/blob/main/LICENSE)。
- 上游 `komari-web` 在上述导入版本中没有提供许可证文件。本仓库不为 `web/` 追加或推定许可证；其版权和使用授权仍由原作者及相关权利人决定。
- 本仓库没有声明覆盖全部目录的统一许可证，也没有授予超出各组件原有授权范围的额外权利。
