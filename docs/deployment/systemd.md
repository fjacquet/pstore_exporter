# systemd (EL9 host)

For a non-container deployment on Enterprise Linux 9, use the unit shipped in `deploy/`.

## Install

```bash
# user + binary
sudo useradd --system --no-create-home --shell /usr/sbin/nologin pstore
sudo install -m 0755 bin/pstore_exporter /usr/local/bin/pstore_exporter

# config + secrets
sudo install -d -o root -g pstore -m 0750 /etc/pstore_exporter
sudo install -m 0640 -o root -g pstore config.yaml /etc/pstore_exporter/config.yaml
sudo install -m 0600 -o root -g pstore deploy/pstore_exporter.env.example /etc/pstore_exporter/pstore_exporter.env
# edit /etc/pstore_exporter/pstore_exporter.env to set PSTORE1_PASSWORD=...

# service
sudo install -m 0644 deploy/pstore_exporter.service /etc/systemd/system/pstore_exporter.service
sudo systemctl daemon-reload
sudo systemctl enable --now pstore_exporter
```

Set `logName: ""` in `config.yaml` so logs go to the journal.

## Operate

```bash
journalctl -u pstore_exporter -f         # follow logs
sudo systemctl reload pstore_exporter    # live config reload (sends SIGHUP)
sudo systemctl status pstore_exporter
```

## Hardening

The unit runs as the unprivileged `pstore` user inside a sandbox:

- `NoNewPrivileges=true`, `ProtectSystem=strict`, `ProtectHome=true`
- `PrivateTmp`, `PrivateDevices`, `ProtectKernel*`, `ProtectControlGroups`
- `RestrictAddressFamilies=AF_INET AF_INET6`, `RestrictNamespaces`, `LockPersonality`
- `Restart=on-failure`

Secrets are supplied through the `EnvironmentFile` and referenced as `${PSTORE1_PASSWORD}`
in `config.yaml`. Keep that file mode `0600`.
