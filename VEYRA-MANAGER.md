# Rovunet Panel export formats and core-plan notes

This file began as the "Veyra Manager foundation" notes; Veyra was the project's earlier name.
Based on this repository's earlier VLESS fork history (see git log).
Original LICENSE and upstream attribution retained. Binary/command remains `slipgate`.

Install/share/wizard and managed-user outputs use `rovunet://` (format v32, with collection metadata) for native configurations. Earlier increments used `veyra://`, which the app still accepts as an alias. VLESS paired output retains its standard `vless://` link as well. The native payload keeps DNS and chain settings.

New command:

    slipgate export-clients --user alice --format auto

Formats:
- auto: standard VLESS WS link; native `rovunet://` for other supported transports.
- standard: only transports with an implemented lossless standard exporter (currently VLESS); skip others with stderr explanation.
- rovunet: native `rovunet://` payload (v32) for every supported transport.
- veyra: compatibility alias; the legacy native payload under the `veyra://` prefix, without collection metadata.
- legacy: the original `slipnet://` prefix for old clients.

Only enabled tunnels and an existing user are exported. DNSTT includes normal and NoizDNS variants. This command exports configured tunnels; the installer separately retains its existing Naive backend variants. Output contains credentials; stdout is one URI per line, errors go to stderr. Partial export is allowed; inspect stderr. No exported links is an error.

Non-legacy VLESS output refuses self-signed TLS endpoints to match the Rovunet app's trust validation. Configure a certificate front or CDN. Existing old links using self-signed origins will fail in the app until the endpoint is corrected. Public per-user enrollment, quotas and device-specific WireGuard keys are not implemented.

Validation: `go test ./...` passed; updated install/user-output tests and new lossless payload/unsupported format/TLS tests included. No live server was modified, deployed or load-tested.

## New protocol configuration generation

`core-plan` creates a new mode-0700 directory with server.json and individual
mode-0600 device profiles. Example (replace host, SNI, paths and port):

    slipgate core-plan --protocol reality --host SERVER_IP --sni TLS_TARGET_NAME \
      --target TLS_TARGET_NAME:443 --port 8443 --devices 20 --output ./reality-plan
    slipgate core-plan --protocol xhttp --host vpn.example.org --sni vpn.example.org \
      --cert /path/fullchain.pem --key /path/privkey.pem --port 8444 \
      --devices 20 --output ./xhttp-plan
    slipgate core-plan --protocol hysteria2 --host vpn.example.org --sni vpn.example.org \
      --cert /path/fullchain.pem --key /path/privkey.pem --port 8445 \
      --devices 20 --output ./hy2-plan
    slipgate core-plan --protocol wireguard --host SERVER_IP --port 51820 \
      --devices 20 --output ./wg-plan

Validate with pinned Xray: `xray run -test -c ./reality-plan/server.json`.
The generator creates distinct cryptographic credentials for each device.
WireGuard has a distinct private key and tunnel address per device; do not reuse
one device file concurrently. Its example subnet is 10.77.0.0/16; review overlap
with your networks before deployment.

These commands prepare configurations only. They do not choose an externally
reachable port, issue certificates, change firewall rules, create systemd units,
install Xray or register these servers in the existing tunnel menu. Server setup
and persistent service lifecycle integration remain a separate deployment step.

## Runtime compatibility and CI
Core plans are validated against Xray commit
`63ca2e0c1d31b89f67700c33842431f541fedd25` (26.10.31).
This is the supported baseline; older versions are not validated, including for
`raw`, `xhttp`, Hysteria2 and WireGuard schemas. `TestGeneratedConfigWithXray`
passes each generated server JSON directly to `xray run -test`.
Set `XRAY` to that binary for local tests; CI requires it.
