# 故障排查

[文档](README.md) › 故障排查

先运行 `sudo rovunet diag`，再查看相关隧道的服务日志（`sudo rovunet tunnel logs <tag>`）。

## 服务器

| 现象 | 可能原因 | 解决方法 |
|---|---|---|
| 隧道不是 `active` | 端口、密钥或配置问题 | `sudo rovunet diag`，然后 `sudo rovunet tunnel logs <tag>` |
| 53 端口被占用 | systemd-resolved 的 stub 监听器 | 面板会将其禁用；若再次出现：`sudo systemctl disable --now systemd-resolved`，并在 `/etc/resolv.conf` 中写入 `nameserver 1.1.1.1` |
| 对服务器 `dig` 超时 | 服务商封锁入站 UDP/53 | 更换服务商，或使用 443 端口上的协议 |
| Let's Encrypt 失败（NaïveProxy） | A 记录尚未生效，或 80 端口关闭 | 等待并开放 80 端口，然后 `sudo rovunet restart` |
| 多个 DNS 隧道中只有一个可用 | 路由模式为 single | `sudo rovunet router status`；用 `router mode` 切换 |
| `tunnel add` 拒绝某个 TCP 端口 | 其他程序正在监听 | 换一个端口或停止该程序 |
| `tunnel status` 中证书被标记 | 已过期、即将过期或无法读取 | 续期或替换证书，然后重启隧道 |
| 更新后某服务仍是旧名称 | 迁移被中断 | 再次运行 `sudo rovunet post-update`，重复运行是安全的 |

## 客户端

| 现象 | 可能原因 | 解决方法 |
|---|---|---|
| 能连接但没有流量 | 网络路径或客户端问题 | 在服务器上测试：`curl -x socks5h://USER:PASS@SERVER_IP:1080 https://api.ipify.org`。若在那里可用，说明服务器正常 |
| `Connection was reset` | 过滤在该端口重置连接 | 用 `tcpdump -ni any tcp port 8443` 检查数据包是否到达；若没有，改由 NaïveProxy 前置 |
| 用户无法登录 | 凭据或后端错误 | `sudo rovunet users list`；检查隧道的后端 |
| VLESS 能连接但打不开网站 | 用户已删除或修改了密码 | 分发新的配置 |
| “WebSocket upgrade failed” | 链接中的路径与服务器不一致 | 分发新的配置 |
| Cloudflare 521 / 522 | Cloudflare 无法到达服务器 | 在服务商防火墙中开放端口 |
| Cloudflare 525 / 526 | SSL 模式错误 | 将 SSL/TLS 设为 **Full** |
| v2rayNG 在 VLESS 上报错 | 开启了 Mux 或 XUDP | 关闭 Mux；只支持普通 UDP |
| DNS 隧道很慢 | MTU 对当前路径过大 | `sudo rovunet mtu 1200`（链路极差时用 1100） |
| 不得不更换密钥 | 旧配置仍使用旧密钥 | `sudo rovunet tunnel edit --tag <tag>`，然后分发新配置 |

## 日志

```bash
journalctl -u rovunet-<tag> -n 200 --no-pager
journalctl -u rovunet-dnsrouter -n 200 --no-pager
journalctl -u rovunet-socks5 -n 200 --no-pager
journalctl -u rovunet-core-<name> -n 200 --no-pager
```

问题与讨论：[t.me/RovunetVPN](https://t.me/RovunetVPN)
