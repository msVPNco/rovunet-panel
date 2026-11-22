# Complete setup review candidate

This source candidate builds on public v2.0.2/main. It does not include or
claim to reproduce the unavailable private `v2.0.3-preview.2` source changes.

## Missing managed runtimes

Managed runtime downloads previously used the panel version as their release
tag. A preview built from this public source with `Version=2.0.3-preview.2`
would therefore request assets from `v2.0.3-preview.2`, even though the source's
reviewed runtime hashes identify the published `v2.0.2` engines. This establishes
a defect in the available source; the unavailable private preview may have
additional changes. A successful test using preinstalled lab engines did not
cover this download path.

`binary.RuntimeReleaseTag` now explicitly selects `v2.0.2`, independently of
panel version and update channel. Xray, the dedicated managed Xray, sing-box,
mita, AmneziaWG and its provenance file all use the same selection. Errors
identify the requested asset and runtime release. There is no latest fallback
and no environment override of the compiled production trust root.

Updating the runtime release is a source change: update the selected tag,
reviewed Go/release/supplemental runtime pins, and the authentic production
manifest fixture together, then repeat exact-runtime acceptance. Merely
bumping a panel version does not change its runtime bytes. A fork also needs
its own reviewed trust roots and authenticated runtime assets.

The download tests use a local signed fixture under the runtime tag while
the panel identifies as an unpublished preview. They cover successful
acquisition, missing metadata/asset, invalid signature, wrong tag/repository/
architecture, changed or truncated bytes, and production rejection of a test
signing key. Failed acquisition leaves no staged candidate behind. A separate
test checks all embedded runtime pins against the actual signed production
release manifest for both amd64 and arm64; this is not physical ARM64 testing.

`complete-setup` continues adding missing cores without an unconditional
runtime refresh. Existing users, stable IDs, subscription URLs, stopped cores
and legacy executable choices retain their existing preservation rules.

## Consolidated earlier review changes

This candidate includes the reviewed changes from PRs #66–#69: an empty or
unreadable core inventory is visible in diagnostics, tunnel statistics cover
the full inventory, managed ports are selected randomly from free compatible
ports, and donation accounts use a shared 50 GiB/30-day server quota with
private export. Existing chosen ports and the DNS/HTTPS base ports are kept.

## Slipstream

Public v2.0.2 already serves the shared `TOKEN.trust.json` companion for both
Android and Windows subscriptions. The reported private-preview regression
has not been reproduced from this public source. A new regression test checks
that a missing or invalid public certificate preserves the active account's
previous feed/trust bytes, still retires another revoked account, and resumes
publication with the same identity and URLs after certificate recovery.

## Scope

This is a source review candidate, not a signed production release or a live
VPS repair. No customer server has been modified. Runtime acquisition tests
and source checks do not establish a real client connection over the user's
network; the existing isolated system/runtime acceptance gates remain required
before production promotion.
