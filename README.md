<h1 align="center">Interlinked</h1>

<p align="center">
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white" alt="Vue">
  <img src="https://img.shields.io/badge/Vite-8-646CFF?logo=vite&logoColor=white" alt="Vite">

  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Gin-1.12-00A0A0?logo=gin&logoColor=white" alt="Gin">
  <img src="https://img.shields.io/badge/GORM-1.31-CC0000?logo=go&logoColor=white" alt="GORM">
</p>
<p align="center">
  <img src="https://img.shields.io/badge/MySQL-8.0-4479A1?logo=mysql&logoColor=white" alt="MySQL">
  <img src="https://img.shields.io/badge/Redis-7-FF4438?logo=redis&logoColor=white" alt="Redis">
  <img src="https://img.shields.io/badge/RabbitMQ-3-FF6600?logo=rabbitmq&logoColor=white" alt="RabbitMQ">
</p>
<!-- <p align="center">
  <img src="https://img.shields.io/badge/WebSocket-实时通信-141010?logo=socketdotio&logoColor=white" alt="WebSocket">
  <img src="https://img.shields.io/badge/Docker_Compose-基础设施-2496ED?logo=docker&logoColor=white" alt="Docker Compose">
</p> -->

**Interlinked** 是一个基于 Go + Vue 3 的社交系统，实现了信息流（Feed）、关注关系、互动通知、私信、发现页等核心能力。
后端采用 **Outbox 事件表 + RabbitMQ** 的可靠异步分发，以及**推拉混合**的时间线模式；同时引入了 Redis 缓存、布隆过滤器、分布式锁与 Lua 令牌桶限流等工程化设计。

## 核心功能

- **用户体系**：邮箱验证码注册、登录、JWT 鉴权、资料修改、绑定/更换邮箱、用户搜索
- **关注关系**：关注/取关、粉丝与关注列表、Redis Set 缓存
- **Feed 动态**：发布、删除（级联清理）、转发、点赞、评论/回复、搜索
- **时间线**：推拉混合分发、游标分页、大 V 阈值可配置
- **私信系统**：WebSocket 实时收发 + HTTP 会话/历史兜底，未读数聚合
- **通知中心**：点赞、评论、关注通知，分页与一键已读
- **发现页**：热门推荐
- **运维面板**：MQ 指标、缓存指标观测
- **可靠性**：Outbox + 重试队列 + 死信队列 + 幂等消费 + 熔断降级

## 技术栈

| 层次 | 技术 |
| --- | --- |
| 后端 | Go 1.26、Gin、GORM、gorilla/websocket、golang-jwt、Viper |
| 前端 | Vue 3（Composition API）、Vue Router、Pinia、Axios、Element Plus、Vite |
| 存储 | MySQL 8.0、Redis 7 |
| 消息队列 | RabbitMQ 3（主队列 / 重试队列 / 死信队列） |
| 基础设施 | Docker Compose、SMTP 邮件服务 |

## 项目结构

```text
interlinked/
├── backend/                  # Go 后端（module: feed）
│   ├── main.go               # 启动入口：配置 → MySQL → Redis → RabbitMQ → 路由
│   ├── config.yaml.example   # 配置模板（复制为 config.yaml 使用）
│   ├── config/               # Viper 配置加载
│   ├── router/               # 路由注册
│   ├── middleware/           # JWT 鉴权、CORS、限流
│   ├── handlers/             # HTTP / WebSocket 处理层
│   ├── services/             # 业务逻辑层（Feed、关注、私信、通知、Outbox 等）
│   ├── repository/           # 数据访问层
│   ├── models/               # GORM 模型与数据库初始化
│   ├── cache/                # Redis 缓存、布隆过滤器、分布式锁
│   ├── mq/                   # RabbitMQ 生产者/消费者、重试与死信
│   ├── realtime/             # WebSocket 连接管理与消息广播
│   ├── mail/                 # SMTP 邮件（验证码）
│   ├── utils/                # JWT 工具、统一响应等
│   └── uploads/              # 上传文件存储目录
├── frontend/                 # Vue 3 前端
│   └── src/
│       ├── api/              # Axios 接口封装
│       ├── components/       # 通用组件
│       ├── views/            # 页面（时间线、发现、私信、通知等）
│       ├── stores/           # Pinia 状态管理
│       └── router/           # 前端路由
├── docs/                     # 各模块 API 设计文档
├── docker-compose.yml        # MySQL / Redis / RabbitMQ 一键编排
└── build/                    # 构建产物
```

## 快速上手

### 1. 启动基础设施

```bash
docker compose up -d
```

将启动 MySQL 8.0（3306）、Redis 7（6379）、RabbitMQ 3（5672，管理界面 15672），并带有健康检查与数据卷持久化。

### 2. 配置并启动后端

```bash
cd backend
cp config.yaml.example config.yaml   # Windows 用 copy
```

按需修改 `config.yaml` 中的数据库、Redis、RabbitMQ 连接信息（本地 Docker Compose 默认密码均为 `123456`）。首次启动建议保持 `auto_migrate: true` 自动建表。

```bash
go run main.go
```

后端默认监听 `http://localhost:8080`。

### 3. 启动前端

```bash
cd frontend
npm install
npm run dev
```

访问 `http://localhost:3000`。Vite 已配置 `/api`、`/uploads`、`/ws` 代理到后端 8080 端口，无需处理跨域。

### 4. 邮件验证码（可选）

`email.debug: true` 时不连接 SMTP，验证码直接打印到后端控制台，方便本地调试；接入真实邮箱时修改 `email` 配置段即可。
