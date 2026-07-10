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
| Mac M1 / M2 / M3 / M4 | `*-macos-apple-silicon.tar.gz` |
| Mac Intel 芯片 | `*-macos-intel.tar.gz` |
| Linux 普通电脑 | `*-linux-x86_64.tar.gz` |
| Linux ARM（树莓派等） | `*-linux-arm64.tar.gz` |
| Windows 64 位 | `*-windows-x86_64.zip` |

macOS 若提示无法验证：运行 `xattr -d com.apple.quarantine ./tun-tui`

## 快捷键

| 按键 | 功能 |
|------|------|
| `i` | 输入订阅 |
| `s` | 连接 / 断开 |
| `j` `k` | 选择节点 |
| `Enter` | 确认节点 |
| `m` | 切换模式 |
| `u` | 更新订阅 |
| `t` | 测速 |
| `r` | 重载 |
| `q` | 退出 |

异常退出后无法上网：`sudo tun-tui -cleanup`

## 开发

```bash
make build      # 编译
make run        # 运行（需 sudo）
make release    # 打包到 dist/
```

发布：`git tag v0.1.1 && git push origin v0.1.1`

MIT
