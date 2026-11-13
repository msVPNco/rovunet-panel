# Private data, diagnostics and retention

| Surface | Contents / protection | Retention and operator action |
|---|---|---|
| /etc/rovunet/config.json | Passwords, subscription tokens, account/period IDs; atomic0600 | Live state; encrypted export before retirement, dispose of old copies deliberately |
| Core/tunnel credentials and applied state | Protocol keys and per-user assignments in private owned directories | Needed for stable configs and recovery; do not publish core directories |
| Static subscription/status files | Private client links and account counters; HTTPS capability URL; no directory listing/no-store route | Refresh replaces valid files atomically; deletion/explicit rotation retires old aliases |
| Usage history | Absolute counters, source generations, account/period data | Kept to avoid losing or double-counting traffic; never reset merely to simplify rollback |
| Exported backup | Encrypted age by default; explicit plaintext supported | Operator chooses retention and stores recovery identity separately; test restore with synthetic data |
| Internal setup/update snapshot | Plaintext root-only recovery material | Keep through validated recovery/upgrade; clean only after compatibility and usage reconciliation |
| systemd journal | Process errors, unit/local paths, aggregate DNS resource drops; engine logs can contain credentials/addresses | Host journald retention applies; configure a disk/time budget appropriate to the host |
| Ordinary panel diagnostics | Status, availability, local command for detailed journal | Does not automatically publish raw journal; review manually before sharing |
| dns-resolvers.sh | Explicit opt-in capture of immediate DNS sources; private temp/output files | Off unless invoked; snapshot duration bounded; log/watch require operator retention/space management |
| CI/test evidence | Synthetic users, documentation addresses, ephemeral test authority | Keep public-safe summaries; never include production credentials, raw backups or signing material |

No telemetry/upload service or automatic sharing of diagnostics was added. Export-client and user-config commands deliberately print private links for the operator; their output is not a public diagnostic report.

Resolver capture can reveal a shared recursive resolver or a direct client source. It does not reliably identify a person/device/account. PTR lookup in snapshot/report contacts the configured DNS service; avoid it when that disclosure is inappropriate. The watch command now stops only its own capture/extractor processes rather than its caller's process group.

For an operator who explicitly enables the default resolver log, a suitable starting logrotate policy is below. Review size/retention for the host and install it yourself; this review does not change a live log policy. copytruncate has a small loss window; this is diagnostic sampling, not billing data.

~~~text
/var/log/rovunet-dns-clients.log {
    daily
    maxsize 10M
    rotate 7
    maxage 7
    compress
    missingok
    notifempty
    copytruncate
    su root root
}
~~~

For journald, choose SystemMaxUse/RuntimeMaxUse/MaxRetentionSec in the host's existing configuration. No universal retention is silently applied. Review proxy access-log configuration separately; enabling request-URI logging can expose subscription tokens. Redact credentials, complete URIs/tokens, private addresses and host identifiers from shared evidence.
