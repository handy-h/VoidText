# API 限流器 — 已删除

> 创建日期：2026-05-16
> 状态：✅ **已解决** — 2026-05-16

---

## 处理

全部限流器代码（~109 行）已从 `internal/external/api.go` 中删除：

- `APIRateLimiter` 结构体 + `NewAPIRateLimiter` + `Acquire`
- 全局变量 `remoteRateLimiter` / `localRateLimiter`
- `RateLimiterConfig` + `DefaultRateLimiterConfig`
- `initRateLimiters` / `getRemoteRateLimiter` / `getLocalRateLimiter`
- `doRequestWithRetry` 中的 `Acquire()` 调用点

限流功能已被 `RetryConfig` + `ExponentialBackoff` 的后置重试机制替代（遇到 HTTP 429 指数退避）。
