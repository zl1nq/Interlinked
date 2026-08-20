# FeedLink Frontend

FeedLink 前端基于 Vue 3 + Vite 实现，负责社交系统的页面渲染、路由跳转、API 调用、登录态维护和 WebSocket 私信交互。

## 技术栈

- Vue 3 Composition API
- Vue Router
- Pinia
- Axios
- Element Plus
- Vite

## 本地启动

```bash
npm install
npm run dev
```

默认端口：

```text
http://localhost:3000
```

后端默认地址：

```text
http://localhost:8080
```

Vite 代理配置位于 `vite.config.js`，当前 `/api` 会代理到后端。

## 环境变量

WebSocket 默认连接当前前端域名下的 `/api/ws/messages`：

```text
ws://localhost:3000/api/ws/messages?token=xxx
```

如果需要直连后端，可以设置：

```env
VITE_WS_BASE_URL=ws://localhost:8080
```

## 页面模块

- 登录 / 注册
- 首页时间线
- 发现页
- 发布动态
- 动态详情
- 搜索结果
- 个人主页
- 通知中心
- 私信页面
- 运维面板

## 私信页面说明

私信页面位于 `src/views/Messages.vue`。

当前实现：

- WebSocket 负责实时发送、接收、ack 和 `message:new` 推送。
- HTTP 兜底接口负责会话列表和历史消息读取。
- 如果 WebSocket 未连接，联系人与历史仍可展示；只有实时发送需要等待 WS 重连。
- 会话列表使用后端 `conversations` 聚合表返回的 `unread` 展示未读数。

相关 API：

- `GET /api/messages/conversations`
- `GET /api/messages/history/:target_id`
- `GET /api/ws/messages?token=xxx`

## 构建

```bash
npm run build
```

当前构建可能出现 Vite chunk 体积提示，这是警告，不影响构建产物。后续可以通过路由懒加载和手动分包优化。
