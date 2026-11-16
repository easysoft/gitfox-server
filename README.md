<p align="center"><a href="https://www.gitfox.net" target="_blank" rel="noopener noreferrer"><img src="docs/gitfox.svg" width="300" alt="GitFox Logo"></a></p>

<p align="center">
  <a href="https://github.com/easysoft/gitfox-server/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-AGPL%20or%20ZPL-blue" alt="GitFox License"></a>
  <a href="https://www.gitfox.net"><img src="https://img.shields.io/badge/Website-www.gitfox.net-blue" alt="GitFox Website"></a>
</p>

# 介绍

GitFox 是一个用 Go 编写的一体化代码托管与 DevOps 平台，提供 Git 仓库托管、代码评审、CI/CD 流水线、制品库、代码扫描等能力，可与禅道（ZenTao）等项目管理/研发流程系统集成。

前端项目地址：[https://github.com/easysoft/gitfox-front](https://github.com/easysoft/gitfox-front)。


## 功能特性

- **代码托管**：仓库、分支、标签、提交、合并请求（Pull Request）、Webhook、SSH/HTTP 协议访问
- **代码评审**：合并请求评审流程、分支保护规则、Code Owner
- **CI/CD**：内置流水线（pipeline）引擎与 Runner（支持 Docker），支持 Cron 定时任务
- **制品库**：容器镜像、Helm、Raw 等制品存储与回收站/GC
- **代码扫描**：规则/规则集/扫描方案/扫描计划管理与扫描任务
- **空间与权限**：多空间（Space）、RBAC 权限、SSO/Token 认证
- **仓库导入**：从其他平台导入/镜像同步仓库
- **国际化**：中英文界面（zh-CN / en-US）

## 技术栈

- 语言：Go 1.25
- HTTP：Gin
- 依赖注入：Google Wire
- ORM：GORM（MySQL / GaussDB / SQLite / 达梦）
- 任务编排：内置 Job Scheduler
- 构建：Taskfile（`task`）

## 快速开始

### 1. 使用 Docker Compose 快速部署

需要安装 Docker 与 Docker Compose。

```bash
git clone <本仓库地址>
cd gitfox-community
docker compose -f deploy/docker-compose.yaml up -d
```

`deploy/docker-compose.yaml` 会启动 MySQL（`zentao-db`）、Web 前端（`zentao`）与 `gitfox` 服务。部署前请按实际情况修改：

- `GITFOX_SERVER_HOST`：对外访问的域名/IP
- `GITFOX_DEVOPS_URL`：禅道服务地址（`http://<禅道地址>/api.php/v1`）
- 各服务使用的镜像地址、数据库密码与数据卷路径

默认映射端口：HTTP `3000`、SSH `3322`。启动后访问 `http://<服务器地址>:3000` 完成初始化。

> 镜像默认指向内部仓库，公开部署时请替换为可访问的镜像地址。

### 2. 编译（从源码构建）

```bash
git clone <本仓库地址>
cd gitfox-community
task build          # 产物位于 bin/gitfox_linux
```

### 3. 配置

配置支持 `yaml` 与 `env` 两种形式，示例见 `configs/prod.yaml`、`configs/prod.env`。

关键环境变量：

| 变量 | 说明 |
| --- | --- |
| `GITFOX_WORKSPACE_ROOT_DIR` | 安装根目录，默认 `/data/gitfox-community` |
| `GITFOX_SERVER_PROTO` | 对外访问协议：`http` / `https` |
| `GITFOX_SERVER_HOST` | 对外访问域名/IP，用于生成仓库与 Webhook 等地址 |
| `GITFOX_HTTP_PORT` / `GITFOX_SSH_PORT` | HTTP / SSH 端口 |
| `GITFOX_REPO_URL_EXPOSE_PORT` | 仓库克隆地址是否附带端口 |
| `GITFOX_DEVOPS_URL` | 禅道 API 地址，格式 `http://<禅道地址>/api.php/v1` |
| `GITFOX_DB_DRIVER` | 数据库驱动：`mysql` / `sqlite` / `gaussdb` / `dm` |
| `GITFOX_DB_HOST` | 数据库主机地址 |
| `GITFOX_DB_PORT` | 数据库端口 |
| `GITFOX_DB_USER` | 数据库用户名 |
| `GITFOX_DB_PASSWORD` | 数据库密码 |
| `GITFOX_DB_NAME` | 数据库名称 |
| `GITFOX_DB_TIMEZONE` | 数据库时区，默认 `Asia/Shanghai` |
| `GITFOX_DB_DATASOURCE` | 完整数据源（DSN），达梦等驱动需要 |
|
| `GITFOX_CI_PLUGINS_ZIP_URL` | CI 插件包下载地址 |
| `GITFOX_LOG_LEVEL` | 日志级别，-1~5（debug~fatal） |
| `GITFOX_DOCKER_HOST` | Docker daemon 地址，默认 `unix:///var/run/docker.sock` |


> 首次启动会自动执行数据库迁移。请勿使用示例中的占位密码。

### 4. 启动

```bash
./bin/gitfox_linux server prod
```

### 5. Docker

```bash
docker build -f deploy/Dockerfile -t gitfox-community .
docker compose -f deploy/docker-compose.yaml up -d
```

> `deploy/Dockerfile` 与流水线/扫描相关的容器镜像默认指向内部镜像仓库，公开部署时请替换为可访问的镜像地址（见下节）。

## 容器镜像说明

流水线执行与代码扫描会拉取一批容器镜像。当前代码中的默认镜像地址为内部仓库，社区部署时需按需替换：

- 语言/构建基础镜像（golang、node、gradle、alpine、docker-cli 等）可替换为 Docker Hub 官方镜像
- 插件类镜像（git/scanner/uploader/pipeline-helper 等）需替换为你自建的镜像地址

相关位置：`pkg/utils/common/const.go`、`pkg/pipeline/step/base/*.go`、`pkg/runner/docker/engine2/compiler/*.go`、`resources/pipeline/codescan*.yaml`。

## 开发指南

```bash
task test:service     # 运行 Service 层单测并生成覆盖率
go build ./...        # 全量编译
go vet ./...          # 静态检查
```

- 代码结构：`app/`（业务：api/controller、service、store、model、pipeline 等）、`pkg/`（通用组件）、`cmd/`（入口）、`resources/`（内置资源）
- 依赖注入：修改 provider 后运行 `wire ./cmd/cli/operations/server` 重新生成
- 提交规范：`[类型](模块): 描述`，类型见 `CONTRIBUTING.md`

## 许可证

本项目采用 **ZPL 1.2 / AGPL 3.0 双许可**，你可任选其一。详见 [LICENSE](./LICENSE)。

Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)

### 第三方开源代码

本项目部分代码派生自其他开源项目，相关文件**保留其原始版权与许可头**，具体如下：

| 来源 | 许可 | 主要位置 |
| --- | --- | --- |
| Harness, Inc. | Apache License 2.0 | `pkg/git/`、`pkg/livelog/`、`pkg/stream/`、`pkg/ssh/` 等 |
| Drone.IO Inc. | Polyform License（以源文件头为准） | `pkg/runner/docker/engine2/`、`pkg/pipeline/spec/parse/` 等 |
| Docker, Inc. | Apache License 2.0 | `pkg/runner/docker/internal/docker/stdcopy/` |

本项目还依赖 `go.mod` 中列出的第三方 Go 模块，各模块遵循其各自的许可协议。分发本项目时，请一并遵守上述第三方许可条款。

## 贡献

欢迎提交 Issue 与 Pull Request，请先阅读 [CONTRIBUTING.md](./CONTRIBUTING.md)。
