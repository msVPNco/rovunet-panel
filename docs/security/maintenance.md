# Maintenance acceptance

For each runtime change, record upstream source commit, module replacements, build toolchain/compiler and digest, license/notices, artifact hash/size/architecture and advisory snapshot. Inspect source and release notes; a tag or larger version is not evidence of safer behavior. Keep original and patched source archives and reproducible patch digests when a maintained patch is required.

Run source/binary advisory tools appropriate to the language, triage actual package/symbol/call exposure and report unsupported/devel components explicitly. Review stable pins periodically as part of maintenance and before each release; no unattended upgrade or automatic scan schedule was created by this task.

The CI now pins actions to commits, uses Go1.27.4 and fixed ShellCheck/shfmt/govulncheck versions, checks all shell scripts, retains preview rollback/tag suites and exercises ephemeral-authority bootstrap, actual subscription path containment, encrypted backup CLI, AWG data, complete setup and a DNS systemd restart. workflow_dispatch publication requires reviewed production gates, an exact stable tag, complete signed assets and a compiled production root. CI edits are locally checked; GitHub has not run this private branch yet.

For a supported-protocol claim, preserve configuration options and reject unsupported effective settings with a reason. Record authentication, metering, quota eligibility, established-stream stop, upgrade/backup/restore and per-account collateral effects separately. Hold new protocol experiments behind explicit acceptance; do not add a URI label as a substitute for a maintained server/client implementation.

Local record values should distinguish attempt counts, failures, latency quantiles, session duration/disconnects, reconnect, CPU/RAM and idle/active/screen-off battery/data. Unmeasured values stay null. Run load/fault tests only in owned isolated environments and never use a production resolver as a flood target.
