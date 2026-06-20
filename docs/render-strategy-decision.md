# 渲染策略决策

## 决策

采用 **单一 PNG 策略**：

- `internal/render` 生成 PNG
- Kindle 通过 FBInk 子进程显示（后续集成）
- 浏览器预览通过 `<img src="/frame">` 显示同一 PNG

## 阶段 0 结论

- Cursor API 分页为 `page` + `pageSize`（1-based），事件无独立 `event_id`，使用 payload SHA256 前 16 字节作为去重键
- `usage-summary` 提供 `autoPercentUsed` / `apiPercentUsed` / `totalPercentUsed`
- 图表库选用 `wcharczuk/go-chart/v2`，输出 PNG 子图嵌入主画布
- 文本暂用 `basicfont`（ASCII）；中文标签在 Kindle 真机需后续内嵌 Noto 字体

## 预案 B

若 FBInk PNG 显示不可行，拆分 `render/bitmap` 与 `render/fbink`，共用 `Layout` 数据结构。
