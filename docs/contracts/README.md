# Compatibility contract v1

The current handoff is [Panel–client integration contract v1](panel-integration-v1.md).
It defines capabilities and stable wire semantics independently of changing app
versions. The integration agent must retest the exact final artifacts from all
three app agents; the versions below are historical evidence only.

The capability snapshot is out of band, not an HTTP endpoint or negotiation
protocol. Strict subscriptions retain only version/name/items and id/link; do
not add min_app_version, device IDs or capability fields. The full release
exports 24 records in both feeds. AWG/SS2022 use lossless standard URIs so older
clients can skip unsupported entries safely; the other 22 native items and CLI
exports retain their envelopes. Saved URLs/tokens and item IDs remain.

The [schema](subscription-v1.schema.json) is a review aid, not an execution of a
client importer. [Historical capability evidence](capabilities-v1.json) and
[November13 acceptance](../engineering-acceptance-2026-11-13.md) retain their
original dates/artifacts/counts. They must not be treated as current design
limits or physical-device certification. Missing measurements are null.

Server/account/period IDs retain their existing meaning. Native collection
grouping and position-derived extra-address suffixes still need an explicit
migration before being redesigned; do not rotate tokens or IDs to fit a UI.

No client-dependent panel change is enabled for a joint public release before
the affected apps are ready and integration accepts their final artifacts.
AnyTLS/TUIC/Mieru and an exact simultaneous-device cap are not activated.
