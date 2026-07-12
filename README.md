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

## 订阅链接

首次使用需配置机场订阅。在 TUI 中按 `l` 打开链接管理，可添加多个订阅并切换使用：

| 按键 | 功能 |
|------|------|
| `l` | 打开 / 关闭链接管理 |
| `i` / `a` | 添加新链接 |
| `Enter` | 使用选中的链接 |
| `d` | 删除选中的链接 |
| `j` / `k` 或 `↑` / `↓` | 在列表中移动 |

链接保存在数据目录的 `subscription.links`（当前使用的链接以 `*` 标记），同时会同步写入 `subscription.url` 以兼容外部工具。

## 快捷键

| 按键 | 功能 |
|------|------|
| `S` | 连接 / 断开 |
| `J` / `K` | 选择节点 |
| `Enter` | 确认节点 |
| `M` | 切换模式（分流 / 全局 / 直连） |
| `T` | 测速 |
| `U` | 更新订阅 |
| `L` | 管理订阅链接 |
| `P` | 设置（数据目录 / Git） |
| `Q` | 退出 |

列表超出屏幕时可用方向键滚动。

## 分流（RULE）

内嵌 GeoIP / GeoSite 数据库（编译进二进制，无需下载）。分流模式下按 Clash/Mihomo 常见策略：

| 流量 | 动作 |
|------|------|
| 局域网 / 私有域名 | 直连 |
| 广告域名（`category-ads-all`） | 拦截 |
| 国内域名 / 中国 IP | 直连 |
| 其余 | 走你选的代理节点 |

按 `m` 可在 分流 / 全局 / 直连 之间切换。HUD「规则 开」表示地理数据已就绪。

订阅地址仅允许公网 `http/https`；本机/内网地址会被拒绝。本地控制 API 绑定 `127.0.0.1:9090` 并启用随机 secret。

异常退出后无法上网：`sudo tun-tui -cleanup`

## 开发

```bash
make build      # 编译
make run        # 运行（需 sudo）
make release    # 打包到 dist/
```

发布：`git tag v0.1.8 && git push origin v0.1.8`

MIT
