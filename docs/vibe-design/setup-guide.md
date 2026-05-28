# Vibe Design 集成指南

## 概述

Vite Design 是一个设计覆盖层工具，允许在浏览器中点击元素并描述修改需求，然后 Claude Code 自动编辑代码。

## 安装

```bash
cd website/frontend
npm install --save-dev @bozhidar003/vibe-design
npx vibe-design init
```

初始化后会生成：
- `.vibe/config.json` — 服务器配置
- `.vibe/skills/` — 设计技能文件
- `.claude/skills/vibe-design/` — Claude Code 技能

## 配置

### 基础配置（`.vibe/config.json`）

```json
{
  "port": 2337,
  "autoTrigger": true,
  "proxy": {
    "/api/v1": {
      "target": "http://localhost:6622",
      "changeOrigin": true
    }
  }
}
```

### Proxy 配置（关键）

Vibe Design 原生不支持 proxy，需要修改源码。

#### 问题 1：`http-proxy-middleware` v4 路径剥离

**现象**：`app.use('/api/v1', createProxyMiddleware(...))` 会剥离 `/api/v1` 前缀，导致代理到后端的路径变成 `/config` 而不是 `/api/v1/config`。

**尝试的解决方案**：
```javascript
// 尝试 1：pathRewrite 保留路径 - 失败
app.use(path, createProxyMiddleware({
  target: options.target,
  pathRewrite: { [`^${path}`]: path }  // 不生效，因为 pathRewrite 在剥离后的路径上操作
}));

// 尝试 2：手动 middleware 拦截 - 失败
app.use((req, res, next) => {
  if (req.url.startsWith(path)) {
    createProxyMiddleware({...})(req, res, next);  // v4 API 不兼容
  }
});
```

**最终解决方案**：使用 Node.js 内置 `http` 模块实现代理：

```javascript
import http from "http";
import { URL } from "url";

// 在 createVibeServer 函数中
if (config.proxy) {
  for (const [path, options] of Object.entries(config.proxy)) {
    const targetUrl = new URL(options.target);
    console.log(`[vibe-design] 🔀 Proxy: ${path} -> ${options.target}`);
    app.use((req, res, next) => {
      if (req.url.startsWith(path)) {
        const proxyReq = http.request({
          hostname: targetUrl.hostname,
          port: targetUrl.port,
          path: req.url,  // 保留完整路径
          method: req.method,
          headers: { ...req.headers, host: targetUrl.host }
        }, (proxyRes) => {
          res.writeHead(proxyRes.statusCode, proxyRes.headers);
          proxyRes.pipe(res);
        });
        proxyReq.on("error", (err) => {
          console.error(`[vibe-design] Proxy error: ${err.message}`);
          res.writeHead(502);
          res.end("Proxy Error");
        });
        req.pipe(proxyReq);
      } else {
        next();
      }
    });
  }
}
```

#### 问题 2：修改位置

需要修改 `node_modules/@bozhidar003/vibe-design/dist/dist-QMLUPOFX.js`：

1. **添加导入**（第 1-15 行）：
```javascript
import http from "http";
import { URL } from "url";
```

2. **添加 proxy 中间件**（在 `app.use(express.json(...))` 和 CORS 中间件之后，路由之前）：
```javascript
// 配置 proxy 中间件
if (config.proxy) { ... }
```

## 踩坑记录

### 1. CSP `connect-src 'self'` 跨域问题

**现象**：
```
Content-Security-Policy：由于违反了下列指令："connect-src 'self'"，
此页面位于 http://127.0.0.1:6622/api/v1/config 的资源无法加载
```

**原因**：`API_BASE` 硬编码为 `http://127.0.0.1:6622`，但页面通过 `http://localhost:6622` 访问。CSP 的 `'self'` 只匹配 `localhost:6622`，不匹配 `127.0.0.1:6622`。

**解决方案**：API 请求使用相对路径：
```javascript
// 修改前
const API_BASE = window.__ENV__?.API_BASE || 'http://127.0.0.1:6622';

// 修改后
const API_BASE = window.__ENV__?.API_BASE || '';
```

这样 `fetch('/api/v1/config')` 会自动使用当前页面的 origin。

### 2. `app.use(path, ...)` 路径剥离

**现象**：Express 的 `app.use('/api/v1', middleware)` 会将 `/api/v1/config` 变成 `/config` 再传给中间件。

**解决方案**：使用 `app.use((req, res, next) => {...})` 手动匹配路径，不使用 `app.use(path, ...)` 形式。

### 3. `http-proxy-middleware` v4 API 变化

**现象**：v4 的 `createProxyMiddleware` 在某些用法下不兼容。

**解决方案**：改用 Node.js 内置 `http` 模块，更稳定且无额外依赖。

### 4. `config.json` 中 `pathRewrite: {}` 问题

**现象**：空对象 `{}` 在 JavaScript 中是 truthy 的，导致 `options.pathRewrite || defaultRewrite` 使用了空对象。

**解决方案**：不要在 config.json 中设置 `pathRewrite`，让代码使用默认值。

### 5. 调试 proxy 问题

**调试方法**：
1. 在 proxy 中间件中添加 `console.log` 打印请求路径
2. 检查 config.json 是否被正确读取（注意工作目录）
3. 用 `curl -v` 测试 API 是否正常响应
4. 检查 CSP 头部：`curl -sI http://localhost:6622/ | grep -i content-security`

## 使用方式

```bash
# 终端 1：启动后端
cd /Users/mengshu/Builds/Ashen_protocol
make run

# 终端 2：启动 vibe-design
cd website/frontend
npx vibe-design start
```

浏览器访问 `http://localhost:6622`，按 **Cmd+D** 进入 Design Mode。

## 注意事项

1. **node_modules 修改会丢失**：`npm install` 后需要重新应用 `dist-QMLUPOFX.js` 的修改
2. **端口**：vibe-design 默认运行在 2337 端口，后端在 6622 端口
3. **CORS**：后端已配置允许 `localhost:3000`、`localhost:5173` 等开发服务器端口
4. **API 路径**：所有 API 统一使用 `/api/v1/` 前缀
