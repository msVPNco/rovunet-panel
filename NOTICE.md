# Notice

Rovunet Panel is a modified version of **SlipGate**
(https://github.com/anonvector/slipgate), Copyright (C) 2026 Anonvector.
It is not affiliated with or endorsed by the SlipGate project.

Both SlipGate and Rovunet Panel are licensed under the GNU Affero General
Public License v3.0; see [LICENSE](LICENSE). The complete history of the
original and the modified commits is kept in this repository, and
[CHANGELOG.md](CHANGELOG.md) describes the changes.

The command is `rovunet`, its services are `rovunet-*` and its settings
live in `/etc/rovunet`. Servers installed under the old names are moved to
these on the first `rovunet` (or `slipgate`) command run as root; during a
transition period `slipgate` remains a link to `rovunet` and releases also
carry `slipgate-linux-*` copies so older installs can update. The Go module
path and the `slipnet://` import format keep their original names for
compatibility. Transport binaries (dnstt-server, caddy-naive and others) are
downloaded from the upstream SlipGate releases.

Third-party components keep their own licenses, including Xray-core
(MPL-2.0), Caddy with forwardproxy (Apache-2.0) and the DNS tunnel servers.
