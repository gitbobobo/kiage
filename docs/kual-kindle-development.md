# Kindle KUAL 扩展开发调研

越狱 Kindle 上通过 **KUAL**（Kindle Unified Application Launcher）启动自用软件的技术调研。

## 一、整体架构

**KUAL** 是越狱社区的标准启动器，本身不负责业务逻辑，只负责：

1. 扫描 `/mnt/us/extensions/` 下的扩展
2. 解析 `config.xml` + `menu.json` 生成菜单
3. 用户点击后执行 `action` 指定的脚本/二进制

核心原则：**你的程序不必知道 KUAL 的存在**，KUAL 只是入口。

```
用户点击 KUAL → 选择菜单项 → 执行 shell 脚本 / 二进制
                                    ↓
                          你的实际业务逻辑（LIPC / FBInk / 网络等）
```

### 参考文档

- [KUAL Wiki](https://wiki.mobileread.com/wiki/KUAL)
- [KUAL What's New（扩展开发）](https://wiki.mobileread.com/wiki/KUAL_What%27s_New)

## 二、前置条件

| 项目 | 说明 |
|------|------|
| 越狱 | 必须，方法因机型/固件而异，见 [KindleModding](https://kindlemodding.org/) |
| KUAL 本体 | FW 5.x 较新机型用 **Booklet 版**（FW ≥ 5.9 不支持 Kindlet）；通过 MRPI 安装 |
| MKK | 老设备需要 Mobileread Kindlet Kit；新越狱 hotfix 通常已内置 |
| MRPI | 安装 `.bin` 包（如 USBNetwork）时常用 |

### 关键路径（USB 根目录）

- 扩展目录：`/mnt/us/extensions/<你的应用名>/`
- 包安装目录：`/mnt/us/mrpackages/`

## 三、KUAL 扩展开发（最小可行方案）

每个扩展是一个独立文件夹，**至少两个文件**。

### 1. `config.xml` — 注册扩展

```xml
<?xml version="1.0" encoding="UTF-8"?>
<extension>
    <information>
        <name>我的应用</name>
        <version>1.0</version>
        <author>你的名字</author>
        <id>myapp</id>  <!-- 建议与文件夹名一致 -->
    </information>
    <menus>
        <menu type="json" dynamic="true">menu.json</menu>
    </menus>
</extension>
```

### 2. `menu.json` — 定义菜单与动作

```json
{
    "items": [
        {
            "name": "启动我的应用",
            "priority": 0,
            "action": "./bin/start.sh",
            "exitmenu": false,
            "refresh": false,
            "status": false
        },
        {
            "name": "设置",
            "priority": 1,
            "items": [
                {
                    "name": "查看状态",
                    "action": "./bin/status.sh",
                    "params": "--verbose"
                }
            ]
        }
    ]
}
```

### `menu.json` 常用字段

| 字段 | 默认值 | 作用 |
|------|--------|------|
| `name` | — | 菜单显示文字 |
| `action` | — | 要执行的命令/脚本（相对扩展目录） |
| `params` | — | 追加给 action 的参数 |
| `priority` | 0 | 排序，负数越靠前 |
| `items` | — | 子菜单 |
| `exitmenu` | `true` | `false` 时点击后不关闭 KUAL |
| `refresh` | `false` | `true` 时执行后 1.5s 重载菜单（动态菜单） |
| `status` | `true` | `false` 时不在底部状态栏显示 action |
| `if` | — | 条件显示（按机型/固件过滤） |

**注意**：KUAL 2.x 要求 **严格合法 JSON**（不能用 tab，用 `\t`；注意大小写和换行符）。

### 3. 推荐目录结构

```
/mnt/us/extensions/myapp/
├── config.xml
├── menu.json
├── bin/
│   ├── start.sh      # 启动脚本（需 chmod +x）
│   ├── stop.sh
│   └── myapp         # 可选：交叉编译的二进制
├── etc/
│   └── config        # 配置文件
└── cache/            # 运行时缓存
```

## 四、应用实现方式选型

KUAL 只负责启动，实际程序可选以下路径。

### 方案 A：Shell 脚本（推荐入门）

- 最简单，社区扩展大多如此
- 可直接调用系统工具：`lipc-get-prop`、`eips`、`sqlite3` 等
- 长驻进程时设 `exitmenu: false`，并在脚本里处理退出

```bash
#!/bin/sh
# bin/start.sh

# 禁止屏保
lipc-set-prop com.lab126.powerd preventScreenSaver 1

# 在屏幕显示文字（老接口）
eips 1 1 "Hello from my app"

# 或调用你自己的二进制
exec ./myapp "$@"
```

### 方案 B：Python

- 部分扩展（如 Dashboard）用 Python
- 需先安装 Python（通过 MRPI 或社区包）
- 适合数据处理、网络请求、定时任务

### 方案 C：原生 C/C++（性能与 UI 更好）

- Kindle 是 **ARM Linux**（内核 2.6+），可交叉编译 ELF
- 工具链：[koxtoolchain](https://github.com/KindleModding/koxtoolchain) + [kindle-sdk](https://kindlemodding.org/kindle-dev/kindle-sdk.html)
- 显示：[FBInk](https://github.com/NiLuJe/FBInk)（e-ink framebuffer 绘制，社区事实标准）
- C++ 建议 **静态链接 libstdc++**（工具链 GCC 比设备新）

| 固件 | 工具链 target |
|------|---------------|
| FW < 5.16.3 | `kindlepw2` |
| FW ≥ 5.16.3 | `kindlehf` |

### 方案 D：Kindlet（Java，一般不推荐自用）

- 官方 Kindle Active Content 路线，需签名
- FW ≥ 5.9 新机型 Kindlet 已不可用
- 自用软件优先 Shell / 原生，不必走 Kindlet

### 方案 E：Scriptlets（KUAL 的替代入口）

- 把 `.sh` 放到 `/mnt/us/documents/`，会出现在书库
- 适合极简脚本；复杂应用仍建议 KUAL 扩展

参考：[Scriptlets 文档](https://kindlemodding.org/kindle-dev/scriptlets.html)

## 五、Kindle 系统 API（开发时常用）

### LIPC — 系统 IPC（基于 DBus）

```bash
# 读属性
lipc-get-prop com.lab126.powerd battLevel

# 写属性（如禁止屏保）
lipc-set-prop com.lab126.powerd preventScreenSaver 1

# 启动系统应用
lipc-set-prop com.lab126.appmgrd start app://com.lab126.booklet.home

# 探测可用服务
lipc-probe -a -v
```

文档：[KindleModding - Apps & Services](https://kindlemodding.org/kindle-apps-and-services/index.html)

### 显示

| 工具 | 用途 |
|------|------|
| `eips` | 固件自带，简单文字/对话框 |
| **FBInk** | 功能完整：文字、图片、局部刷新，推荐 |
| GTK+2 | 可做 GUI，见 [GTK 教程](https://kindlemodding.org/kindle-dev/gtk-tutorial/setting-up.html) |

竖屏旋转（Oasis）：kiage 通过加速度计 `/dev/input` 事件监听竖屏正立/倒立切换；`FBINK_NO_SW_ROTA=1` 下 framebuffer 已随设备旋转，渲染时在正立（rota=0）翻转 PNG 180° 以匹配屏幕；触摸映射由 `fbink -e` + quirk 同步。注意 fbink CLI 的 `-r` 是文本右对齐，不可用于旋转。

### 文件系统

- 用户数据：`/mnt/us/`（USB 可见，可读写）
- 系统分区：默认只读，修改需 `mntroot rw`，改完 `mntroot ro`
- **扩展和脚本放 `/mnt/us/`，不要改系统分区**

## 六、开发与调试工作流

```
1. 本机写代码 → 交叉编译（如需要）
2. 复制到 /mnt/us/extensions/myapp/
3. 在 Kindle 上打开 KUAL 测试
4. 安装 USBNetwork / usbnetlite → SSH 调试
```

### SSH 调试（强烈建议）

1. 安装 [USBNetwork](https://wiki.mobileread.com/wiki/USBNetwork) 或 [usbnetlite](https://kindlemodshelf.me/usbnetlite)
2. KUAL 中开启 USB Networking
3. 电脑配置 IP `192.168.15.201`，SSH 到 `root@192.168.15.244`
4. 建议设置 root 密码、配置 SSH key，并开启 Wi-Fi SSH

### 动态菜单（进阶）

脚本可改写 `menu.json` 或 `menu.state*`，配合 `"refresh": true` 实现开关、子菜单切换。参考 [KUAL What's New 动态菜单示例](https://wiki.mobileread.com/wiki/KUAL_What%27s_New)。

## 七、Hello World 完整示例

在电脑上创建后复制到 Kindle。

**`config.xml`** — 见第三节。

**`menu.json`**

```json
{
    "items": [
        {
            "name": "Hello World",
            "priority": 0,
            "action": "./bin/hello.sh",
            "exitmenu": true,
            "status": false
        }
    ]
}
```

**`bin/hello.sh`**

```bash
#!/bin/sh
BATT=$(lipc-get-prop com.lab126.powerd battLevel 2>/dev/null | tr -d '[]')
eips 2 2 "Hello Kindle!"
eips 2 4 "Battery: ${BATT}%"
eips 2 8 "Press any key..."
read -n 1
```

部署后执行 `chmod +x bin/hello.sh`，重启 KUAL 或重新打开即可看到菜单项。

## 八、常见坑

1. **解压破坏**：macOS/Windows 解压可能改大小写或 CRLF，导致脚本随机失败
2. **JSON 语法**：KUAL 2.x 对非法 JSON 零容忍
3. **权限**：脚本必须 `chmod +x`
4. **长驻进程**：设 `exitmenu: false`，并提供停止入口
5. **屏保**：长任务需 `lipc-set-prop com.lab126.powerd preventScreenSaver 1`
6. **固件差异**：新机型用 Booklet 版 KUAL，交叉编译选对 `kindlehf` / `kindlepw2`
7. **禁用扩展**：将 `config.xml` 重命名为 `config-skip.xml` 即可

## 九、参考资源

| 资源 | 链接 |
|------|------|
| KUAL 主帖 | [MobileRead KUAL v2.7](https://www.mobileread.com/forums/showthread.php?t=203326) |
| 扩展开发 Wiki | [KUAL What's New](https://wiki.mobileread.com/wiki/KUAL_What%27s_New) |
| 扩展列表 | [KUAL Extensions](https://www.mobileread.com/forums/showthread.php?t=205064) |
| 开发社区文档 | [kindlemodding.org](https://kindlemodding.org/) |
| 复杂扩展示例 | USBNetwork、linkss（屏保）源码 |
| Python 扩展示例 | [Kindle Dashboard](https://blog.4dcu.be/diy/2020/09/27/PythonKindleDashboard_1.html) |
| 交叉编译 | [koxtoolchain](https://github.com/KindleModding/koxtoolchain) + [kindle-sdk](https://kindlemodding.org/kindle-dev/kindle-sdk.html) |
| 显示库 | [FBInk](https://github.com/NiLuJe/FBInk) |

## 十、实施建议

对「自用 + 已越狱 + KUAL 启动」场景，建议路径：

1. **第一阶段**：Shell + `menu.json` 搭 KUAL 扩展骨架，用 `eips` / LIPC 验证
2. **第二阶段**：装 USBNetwork，SSH 远程迭代
3. **第三阶段**：按需求选 Python（快）或 C + FBInk（显示好、性能好）
