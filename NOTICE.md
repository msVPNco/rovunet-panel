# Notice

Rovunet Panel is a modified version of **SlipGate**
(https://github.com/anonvector/slipgate), Copyright (C) 2026 Anonvector.
It is not affiliated with or endorsed by the SlipGate project.

Both SlipGate and Rovunet Panel are licensed under the GNU Affero General
Public License v3.0; see [LICENSE](LICENSE). The complete history of the
original and the modified commits is kept in this repository, and
[CHANGELOG.md](CHANGELOG.md) describes the changes.

Since v1.8.0 the command, services, system users and files use the Rovunet
name (`rovunet`, `rovunet-*` services, `/etc/rovunet`); servers installed under
the original names are migrated in place on update. The `slipnet://` client
format is still accepted for compatibility with older app versions. Transport
binaries (dnstt-server, slipstream-server, vaydns-server, caddy-naive) are
downloaded from the upstream SlipGate releases.

## Upstream and third-party projects

Third-party components keep their own licenses.

| Project | Role in Rovunet Panel | License |
|---|---|---|
| [SlipGate](https://github.com/anonvector/slipgate) by Anonvector | The project Rovunet Panel is based on | AGPL-3.0 |
| [dnstm](https://github.com/net2share/dnstm) by net2share | The DNS tunnel manager SlipGate was originally built on | — |
| [dnstt](https://www.bamsoftware.com/software/dnstt/) by David Fifield, via [tladesignz/dnstt](https://github.com/tladesignz/dnstt) | DNS tunnel behind the DNSTT and NoizDNS transports | Public domain / CC0 |
| [Slipstream](https://github.com/EndPositive/slipstream) by EndPositive and [slipstream-rust](https://github.com/Mygod/slipstream-rust) by Mygod | QUIC-over-DNS tunnel behind the Slipstream transport | Apache-2.0 |
| [VayDNS](https://github.com/net2share/vaydns) by net2share | KCP-based DNS tunnel behind the VayDNS transport | CC0 |
| [Soar](https://github.com/getlantern/soar) by Lantern | Multipath DNS tunnel, built into the panel | Apache-2.0 |
| [Xray-core](https://github.com/XTLS/Xray-core) by Project X | Xray cores (`core-install`); the VLESS protocol, implemented independently for the built-in VLESS server | MPL-2.0 |
| [Caddy](https://caddyserver.com/) with [forwardproxy](https://github.com/klzgrad/forwardproxy) | NaïveProxy server | Apache-2.0 |
| [dnstun-ezpz](https://github.com/aleskxyz/dnstun-ezpz) by aleskxyz | Inspiration for the WARP integration | — |

## Panel binary dependency notices

The exact module-root license and NOTICE texts for the dependencies linked into
the reviewed Linux binaries, including age, HPKE, Cobra, pflag, Soar and Go libraries, are
included in [third_party_licenses](third_party_licenses/manifest.json). Retain
them with redistributed panel binaries. age/HPKE backup encryption does not
change those projects' own licenses. These notices do not resolve the separate
legacy executable source/license reconstruction gate in
[the runtime inventory](docs/security/runtime-inventory.md).
