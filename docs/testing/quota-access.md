# Checks for traffic-limited users (v1.18.0)

Nothing in v1.18.0 has been run against a real server yet: the code is covered by unit tests that replace systemd, Xray and the account commands. Run these on a **throwaway server** before relying on it.

`scripts/qa-quota-access.sh` checks the server's files and accounts (SSH account, SOCKS5 credentials, Caddyfile, core client files, pending removals). The checks below need a real client.

## 1. An old configuration stops working

1. Create an unlimited user, `sudo rovunet users add`, and keep every configuration that `users configs` prints: Hysteria 2, WireGuard, NaïveProxy, SSH.
2. Connect with each one. All must work.
3. Stop the user (set a past `expires_at` in `/etc/rovunet/config.json` and run `sudo rovunet expire-users`), then `sudo rovunet users renew` with a quota and the **same password**.
4. Try the saved configurations again. Expected: Hysteria 2, WireGuard, NaïveProxy and SSH are **refused**. WireGuard: check both that the handshake fails and that no data passes (the address assigned to a user can change).
5. Fetch the new subscription and connect with VLESS, REALITY or XHTTP. Expected: works, and the quota counts it.

## 2. An open session ends

Open an SSH session (or a tunnel through the SSH account) as an unlimited user, then renew them with a quota as above. Expected: the session closes within seconds and the account is gone (`id <user>` fails).

## 3. Restore and update

- Take a backup that contains a limited user, restore it on a clean server. Expected: the limited user has no SSH account and is in no Hysteria 2 or WireGuard core.
- On a server that already has a limited user with an SSH account (made by an older version), run `sudo rovunet update`. Expected: the account is removed, the cores that held the user restart once, the user's REALITY/XHTTP/VLESS configs still work.

## 4. A core nobody may use

With only limited users, a Hysteria 2 or WireGuard core must be stopped (`systemctl status rovunet-core-<name>`; `state.json` has `"paused": true`). Add an unlimited user: the core must start. Remove the last user of a REALITY core: it stops; add one: it starts.

## 5. A removal that fails

Break a core on purpose (for example make its certificate path unreadable), then limit a user who has a client file in it. Expected: the command reports the failure, the core is stopped, `sudo rovunet users list` shows "Access removal pending on core …". Fix the fault and wait up to 5 minutes (or run `sudo rovunet expire-users`): the core starts and the note disappears. Other users of that core are disconnected while it is stopped.

## 6. Usage is counted (not new, still unproven)

Move a known amount of data in each direction over REALITY and over XHTTP, and compare `sudo rovunet users list` before and after. This tells you both that the transfer was complete and that the counter matches.
