# VoidText 字体目录

本目录用于存放项目内嵌的开源字体文件。

## 内嵌字体

### 霞鹜文楷 (LXGW WenKai)

- **文件路径**: `lxgwwenkai/lxgw-wenkai-regular.woff2`
- **许可证**: SIL Open Font License 1.1
- **项目地址**: https://github.com/lxgw/LxgwWenKai
- **说明**: 免费开源中文字体，适合长时间阅读

## 添加自定义字体

1. 将 woff2 格式的字体文件放入 `custom/` 目录
2. 通过审核界面的「阅读设置 → 字体 → 添加自定义字体」输入字体 URL 加载
3. 也可通过 CSS Font Loading API 动态加载远程字体

## 注意事项

- 推荐使用 woff2 格式（压缩率高，浏览器支持好）
- `custom/` 目录已被 .gitignore 忽略，不会提交到仓库
- 字体文件首次加载后浏览器会缓存
