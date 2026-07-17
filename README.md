# tun-tui

基于 [Mihomo](https://github.com/MetaCubeX/mihomo) 的 TUN 模式终端 VPN。

## 安装

**macOS / Linux：**

```bash
curl -fsSL https://raw.githubusercontent.com/YeDaChai/tun-tui/main/scripts/install.sh | sh
sudo tun-tui
```

**Windows / 手动安装：** 到 [Releases](https://github.com/YeDaChai/tun-tui/releases) 下载对应压缩包，解压后以管理员身份运行。

| 你的电脑 | 下载文件 |
|----------|----------|
| Mac M1 / M2 / M3 / M4 | `*-macos-apple-silicon-arm64.tar.gz` |
| Mac Intel 芯片 | `*-macos-intel-x86_64.tar.gz` |
| Linux 普通电脑 | `*-linux-x86_64.tar.gz` |
| Linux ARM（树莓派等） | `*-linux-arm64.tar.gz` |
| Windows 64 位 | `*-windows-x86_64.zip` |

macOS 若提示无法验证：运行 `xattr -d com.apple.quarantine ./tun-tui`

## 快捷键

| 按键 | 功能 |
|------|------|
| `S` | 连接 / 断开 |
| `J` / `K` | 选择节点 |
| `Enter` | 确认节点 |
| `M` | 切换模式（分流 / 全局 / 直连） |
| `T` | 测速 |
| `L` | 管理订阅链接 |
| `P` | 设置 |
| `Q` | 退出 |

列表超出屏幕时可用方向键滚动。有订阅时启动会自动连接。

### 订阅链接（`L`）

| 按键 | 功能 |
|------|------|
| `I` / `A` | 添加链接 |
| `Enter` | 使用选中链接 |
| `D` | 删除选中链接 |
| `Esc` | 关闭 |

链接保存在数据目录的 `subscription.links`（当前链接以 `*` 标记），并同步写入 `subscription.url`。

### 设置（`P`）

| 按键 | 功能 |
|------|------|
| `D` | 清理本地数据（订阅、缓存、secret 等） |
| `Esc` | 关闭 |

## 分流（RULE）

内嵌 GeoIP / GeoSite（编译进二进制，无需下载）。分流策略：

| 流量 | 动作 |
|------|------|
| 局域网 / 私有域名 | 直连 |
| 广告域名（`category-ads-all`） | 拦截 |
| 国内域名 / 中国 IP | 直连 |
| 其余 | 走所选代理节点 |

订阅地址仅允许公网 `http/https`；本机/内网地址会被拒绝。本地控制 API 绑定 `127.0.0.1:9090`，使用随机 secret。

异常退出后无法上网：`sudo tun-tui -cleanup`

## 开发

```bash
make build       # 本机编译
make build-all   # 全平台交叉编译
make run         # 编译并以管理员运行（数据目录 ./data）
make release     # 打包到 dist/
make help        # 查看全部目标
```

发布：`git tag v0.2.5 && git push origin v0.2.5`

MIT
