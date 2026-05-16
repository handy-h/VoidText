# removeAdContent() — 已修复

> 创建日期：2026-05-16
> 状态：✅ **已解决** — 2026-05-16

---

## 原问题

`removeAdContent()` 是空实现 `return content`，导致 `AdBlacklist` 配置不生效。

## 修复

函数已改为实际的正则替换实现：

```go
func removeAdContent(content, pattern string) string {
    re := regexp.MustCompile(pattern)
    return re.ReplaceAllString(content, "")
}
```

`rules.json` 中 `ad_blacklist` 配置的广告正则模式现在会实际生效。
