# zyxel-1250-exporter

Prometheus exporter for the Zyxel XGS1250-12 multi-gigabit smart-managed switch.

The switch has no documented HTTP API: the exporter logs in to the web interface exactly like a browser does
(RSA-encrypted password over HTTP, plain password inside TLS over HTTPS, signed `cgi/get.cgi` requests) and
reads the same data the UI displays. Tested against firmware `V2.00(ABWE.0)C0`.

## Metrics

| Subsystem | Examples                                                                                           |
| --------- | -------------------------------------------------------------------------------------------------- |
| `system`  | identity (`zyxel_system_info`), uptime, loop detection, DHCP                                       |
| `port`    | link up, admin state, negotiated speed, rx / tx / CRC error packets, PVID, overheat, copper / fiber |
| `vlan`    | VLAN count, member and tagged port counts, per-port membership (0 none, 1 untagged, 2 tagged)      |
| `lag`     | whether each of the 5 link aggregation groups is configured                                        |

Packet counters are those shown by the switch UI (*System > Per port status*); they restart from 0 when the
switch reboots or when you clear them. Per-port metrics only cover the physical ports (the firmware also
reports one trailing entry per LAG group, which is ignored).

Every collector runs concurrently and reports `zyxel_scrape_collector_success{collector}` and
`zyxel_scrape_collector_duration_seconds{collector}`, so a failing endpoint never hides the others.

## Run

```sh
ZYXEL_URL=https://192.168.1.3 \
ZYXEL_INSECURE_SKIP_VERIFY=true \
ZYXEL_PASSWORD='...' \
zyxel-1250-exporter
```

The switch only has an admin password. It is read from `ZYXEL_PASSWORD` only: there is no flag for it, so it
never shows up in `ps`.

| Flag                           | Environment                  | Default               |
| ------------------------------ | ---------------------------- | --------------------- |
| `--zyxel.url`                  | `ZYXEL_URL`                  | `http://192.168.1.3`  |
| (none)                         | `ZYXEL_PASSWORD`             | required              |
| `--zyxel.insecure-skip-verify` | `ZYXEL_INSECURE_SKIP_VERIFY` | `false`               |
| `--listen`                     | `ZYXEL_LISTEN`               | `:9092`               |
| `--scrape-timeout`             |                              | `10s`                 |

With an `https://` URL, set `ZYXEL_INSECURE_SKIP_VERIFY=true`: the switch only has a self-signed certificate.
Prefer HTTPS: over plain HTTP the password is RSA-encrypted but the rest of the session is not protected.

The exporter keeps one session open on the switch, logs in again by itself when the session expires, and logs
out when it receives `SIGINT` / `SIGTERM`. Requests to the switch are serialized. At most two scrapes run at
the same time; further requests get a `503` until one finishes.

`/metrics` is not authenticated and `--listen` binds all interfaces by default. It exposes the switch MAC and IP
address, so bind it to a trusted interface (for example `ZYXEL_LISTEN=127.0.0.1:9092`) or restrict access to the port.

### Docker

```sh
docker run -d --name zyxel-1250-exporter -p 9092:9092 \
  -e ZYXEL_URL=https://192.168.1.3 -e ZYXEL_INSECURE_SKIP_VERIFY=true -e ZYXEL_PASSWORD \
  ghcr.io/xfoucron/zyxel-1250-exporter
```

### Prometheus

```yaml
scrape_configs:
  - job_name: zyxel
    scrape_interval: 30s
    static_configs:
      - targets: ["localhost:9092"]
```

### Grafana

Import [`dashboards/zyxel.json`](dashboards/zyxel.json) in Grafana (*Dashboards > New > Import*) and pick
your Prometheus data source. The `job` variable selects the scrape job(s) of the exporter.

## Development

```sh
go test -race ./...
golangci-lint run
```
