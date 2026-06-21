# kiage

越狱 Kindle（Oasis）上的 Cursor 用量看板，通过 KUAL 启动。

## 功能

- 套餐等级、重置日期、Total/Composer/API 占比
- 月/年 token 与花费汇总
- 可切换折线图（token/花费）与热力图
- 打开应用期间每 10 分钟自动刷新；手动刷新
- 首次无数据时补全当年历史；日常仅更新今日事件

## 本地开发

```bash
make tidy
make dev
```

浏览器打开 http://localhost:8088

竖屏预览：`KIAGE_PORTRAIT=1 make dev`

## 配置凭据

1. 浏览器登录 https://cursor.com/dashboard/usage
2. 任选其一：
   - **Edge 自动导入（macOS 开发机）**：`pip install browser-cookie3` 后执行 `make import-edge`
   - 编辑 `extension/etc/config.json` 的 `cursor.session_token`
   - 将 token 写入 `extension/etc/import/token`（USB 导入）

## 构建与部署

```bash
make build-kindle
```

将 `extension/` 目录复制到 Kindle `/mnt/us/extensions/kiage/`，执行：

```bash
chmod +x bin/*.sh bin/kiage
```

KUAL 菜单：点击 **Kiage** 启动看板；**双击下方向键**退出应用。

### Kindle 方向键

| 操作 | 动作 |
|------|------|
| ↑ 单击 | 切换提供商（Cursor / GLM） |
| ↑ 双击 | 切换 token / 花费（提供商支持时） |
| ↓ 单击 | 切换设置服务开/关 |
| ↓ 双击 | 退出应用 |

Kindle 需安装 [FBInk](https://github.com/NiLuJe/FBInk)（后续版本将打包 fbink 二进制）。

## 快捷键（浏览器预览）

方向键行为与 Kindle 一致（见上表）。另支持：

| 键 | 动作 |
|----|------|
| Space | 切换折线图 |
| H | 切换热力图 |
| R | 手动刷新 |

## 威胁模型

`/mnt/us` 为 FAT 分区，配置文件 USB 可见。请妥善保管设备与 token。

## 命令

```bash
kiage run    # Kindle 主循环
kiage dev    # 本地预览
kiage fetch  # 单次同步
```
