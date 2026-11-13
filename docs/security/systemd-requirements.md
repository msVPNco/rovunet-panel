# Service requirements before confinement

This inventory describes requirements, not a claim that every unit has been sandboxed. Most existing units still run as root. A lower systemd-analyze security score alone is not an acceptance test.

| Service family | Current identity / requirements | Potential restriction and mandatory test |
|---|---|---|
| DNS router | root, UDP/TCP53, loopback backends, read config/verification material | NoNewPrivileges=yes added; actual systemd start/restart and UDP/TCP payload checked in private namespaces |
| Public/loopback SOCKS | Dedicated service user; credential file and listener; WARP user when enabled | Keep strict auth and per-user session revocation; validate WARP routing before changing identity/caps |
| Naive/Caddy | root or existing WARP service user; low ports, ACME network/storage, certificate and subscription reads | Needs bind capability for unprivileged mode; test ACME renewal, private file access, reload and WARP before changing write paths |
| VLESS/Xray dedicated and legacy | root; configuration/key/certificate reads, listener/API ports, outbound network | Need protocol-by-protocol filesystem/capability and low-port tests; do not blindly apply DNS restrictions |
| DNSTT/VayDNS/Slipstream | root; UDP backend/listener, DNS routing, keys/certificates | Establish exact runtime file writes/capabilities and shared-resolver payload before reducing privileges |
| Soar / StunTLS | root; DNS/SSH/local listener, network and key/certificate reads | Validate direct vs SSH behavior and limits separately; public-key sharing is not per-account auth |
| AWG userspace | root; /dev/net/tun, interface, routes, NET_ADMIN, tool/runtime access | Generic PrivateDevices/capability drops break it; actual TCP/UDP namespace test and reboot recovery required |
| WARP / firewall restore | root; routes, rules, policy routing, NET_ADMIN and distro WireGuard | Full disposable VM required; no broad restrictions from unit-score suggestions |
| Expiry/certificate timers | privileged one-shot tasks updating owned credentials, usage and units | Measure timer delay/overshoot and cross-account impact; validate restart and reboot scheduling |
| SSH daemon | Host-managed administrator service; Rovunet-owned Match Group | Preserve foreign Match/PermitOpen blocks, validate with sshd -t and real forwarding; no global service replacement |

The DNS lab uses an exactly owned rovunet-lab-router-* transient unit, confirms NoNewPrivs=1 on the actual router, performs 100 UDP and 100 TCP requests per run, then repeats after systemctl restart. It does not reboot the shared VM, renew a real certificate or demonstrate a DNS tunnel/carrier connection. Revert only the affected unit and restart only that service if a restriction fails.
