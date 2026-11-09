---
title: Monitoring
description: Sending a site's log and metrics to Grafana, Elasticsearch or another monitoring system - the JSON log, request ids, the Prometheus metrics and a Grafana Alloy setup.
sidebar:
  order: 17
---

Kosatka keeps no graphs or alerts of its own. It writes what a
monitoring system needs in open formats - the log on standard output,
the metrics in the Prometheus text format - and a collector on the
server sends them to where you look at them: Grafana (Loki for the
log, Prometheus or Mimir for the metrics), Elasticsearch,
VictoriaMetrics and the like.

## Turning it on

In the site's config (or the variables in parentheses):

```yaml
log:
  format: json      # LOG_FORMAT: text (the default) or json
  requests: true    # LOG_REQUESTS: a line for every request
metrics:
  listen: 127.0.0.1:9101   # METRICS_LISTEN: unset - no metrics
```

Restart the site, then check:

```bash
curl -s http://127.0.0.1:9101/metrics | head
journalctl -u kosatka@example -n 5 -o cat
```

Several sites on one server: each its own metrics port (`9101`,
`9102`…). The metrics server answers `GET /metrics` only; keep it on
the loopback address, or behind a firewall when the collector runs on
another machine.

## The log

With `format: json` each line is a JSON object; the request lines
(`requests: true`) look like this:

```json
{"time":"2026-11-09T15:04:05.123Z","level":"INFO","msg":"request","method":"GET","path":"/blog/hello","route":"/","status":200,"duration_ms":12.4,"bytes":18234,"user_id":3,"request_id":"9f2c0e1b7a4d5e61"}
```

| Field | |
|---|---|
| `time`, `level`, `msg` | every line: when, `DEBUG`/`INFO`/`WARN`/`ERROR`, what happened |
| `error` | the error, on the lines about one |
| `request_id` | the request the line is about (below) |
| `method`, `path`, `route`, `status`, `duration_ms`, `bytes` | a request line: the path without the query string, the [route](#the-metrics) that answered, the status code, the time taken, the bytes sent |
| `user_id` | a request line: the signed-in user, none for a guest |

The visitors' addresses aren't in the request lines (the web server's
access log has them), nor the query strings. A path holding a secret - a
`{token}` in its route: a password reset, a preview, an unsubscribe
link - is logged as its route (`/preview/{token}`). Still, keep the
site's log where you would keep the web server's, and mind how long a
hosted service keeps it.

### Request ids

Every answer has an `X-Request-Id` header: the id of the request in
the site's log. When the request comes through a trusted proxy
([`trusted_proxies`](../reference/configuration.md#the-server), the
loopback address by default) with an `X-Request-Id` of its own, the site
keeps it, so the web server's log and the site's share the id. With
nginx:

```nginx
proxy_set_header X-Request-Id $request_id;
```

The request lines and the error lines written while answering a page
(content that failed to load, a panic) carry the id.

## The metrics

| Metric | Type | |
|---|---|---|
| `kosatka_http_requests_total{method, route, code}` | counter | the requests answered |
| `kosatka_http_request_duration_seconds{route}` | histogram | the time to answer, from 5 ms to 5 s |
| `kosatka_http_requests_in_flight` | gauge | the requests being answered |
| `kosatka_db_open_connections`, `kosatka_db_in_use_connections`, `kosatka_db_idle_connections`, `kosatka_db_max_open_connections` | gauge | the database connection pool |
| `kosatka_db_wait_count_total`, `kosatka_db_wait_duration_seconds_total` | counter | the times a query waited for a free connection, and how long |
| `kosatka_build_info{version, commit}` | gauge | the engine's release the binary was built from |
| `go_goroutines`, `go_memstats_heap_alloc_bytes`, `go_gc_cycles_total`, `process_start_time_seconds`… | | the Go runtime, under the usual names |

`route` is the address pattern that answered (`/node/{id}`,
`/admin/content`, `/` for content at its own path), so the number of
series stays small; `none` - no route answered (a redirect, the
maintenance page, an address nothing matches). `/healthz` isn't counted.

Queries to start a dashboard with (PromQL):

```text
# Requests a second, by status code
sum by (code) (rate(kosatka_http_requests_total[5m]))

# The share of server errors
sum(rate(kosatka_http_requests_total{code=~"5.."}[5m]))
  / sum(rate(kosatka_http_requests_total[5m]))

# The 95th percentile of the answer time, by route
histogram_quantile(0.95,
  sum by (le, route) (rate(kosatka_http_request_duration_seconds_bucket[5m])))
```

## Grafana with Alloy

[Grafana Alloy](https://grafana.com/docs/alloy/latest/) on the site's
server reads the journal and the metrics and sends them to Grafana
Cloud, or to your own Loki and Prometheus.

1. In Grafana Cloud, open your stack and note, for *Prometheus* and
   *Loki*, the URL to send to and the user (a number). Create an access
   policy with the `metrics:write` and `logs:write` scopes and a token
   for it.
2. [Install Alloy](https://grafana.com/docs/alloy/latest/set-up/install/linux/)
   on the server and let it read the journal:

   ```bash
   sudo usermod -aG systemd-journal alloy
   ```

3. Put the addresses and the token in Alloy's environment file,
   `/etc/default/alloy` on Debian and Ubuntu (readable by root only):

   ```bash
   PROMETHEUS_URL=https://prometheus-….grafana.net/api/prom/push
   PROMETHEUS_USER=123456
   LOKI_URL=https://logs-….grafana.net/loki/api/v1/push
   LOKI_USER=654321
   GRAFANA_CLOUD_TOKEN=glc_…
   ```

4. Write `/etc/alloy/config.alloy` - here for one site, `example`, with
   its metrics on port 9101 (a second site: another `targets` entry
   and another `loki.source.journal`):

   ```hcl
   // The site's metrics.
   prometheus.scrape "kosatka" {
     targets         = [{"__address__" = "127.0.0.1:9101", "site" = "example"}]
     scrape_interval = "60s"
     forward_to      = [prometheus.remote_write.grafana.receiver]
   }

   // The server's: processor, memory, disks, network.
   prometheus.exporter.unix "host" { }

   prometheus.scrape "host" {
     targets         = prometheus.exporter.unix.host.targets
     scrape_interval = "60s"
     forward_to      = [prometheus.remote_write.grafana.receiver]
   }

   prometheus.remote_write "grafana" {
     endpoint {
       url = sys.env("PROMETHEUS_URL")
       basic_auth {
         username = sys.env("PROMETHEUS_USER")
         password = sys.env("GRAFANA_CLOUD_TOKEN")
       }
     }
   }

   // The site's log, from the systemd journal.
   loki.source.journal "example" {
     matches    = "_SYSTEMD_UNIT=kosatka@example.service"
     labels     = {job = "kosatka", site = "example"}
     forward_to = [loki.process.kosatka.receiver]
   }

   // The level as a label, to pick the errors out quickly.
   loki.process "kosatka" {
     stage.json {
       expressions = {level = "level"}
     }
     stage.labels {
       values = {level = ""}
     }
     forward_to = [loki.write.grafana.receiver]
   }

   loki.write "grafana" {
     endpoint {
       url = sys.env("LOKI_URL")
       basic_auth {
         username = sys.env("LOKI_USER")
         password = sys.env("GRAFANA_CLOUD_TOKEN")
       }
     }
   }
   ```

5. Restart Alloy and look at its components page (Alloy's own
   interface, `http://127.0.0.1:12345` on the server) for errors:

   ```bash
   sudo systemctl restart alloy
   journalctl -u alloy -n 50
   ```

In Grafana's *Explore*, the log (LogQL):

```text
{job="kosatka", level="ERROR"}
{job="kosatka"} | json | msg="request" | duration_ms > 1000
{job="kosatka"} |= "9f2c0e1b7a4d5e61"
```

Your own Loki and Prometheus instead of Grafana Cloud: the same
configuration with their addresses (Prometheus started with
`--web.enable-remote-write-receiver`), or a Prometheus scraping
`/metrics` itself.

## Elasticsearch

The Elastic Agent (or Filebeat) reads the journal - the `journald`
input, with the message decoded as JSON - and its Prometheus
integration (or Metricbeat's `prometheus` module) collects `/metrics`.
See Elastic's documentation for those inputs.

## What to alert on

- The site not answering: a check of `https://<site>/healthz` from
  outside the server (it answers `ok` while the database does).
- The share of server errors above, say, 1% for 10 minutes.
- The 95th percentile of the answer time above a second.
- The disk filling up (`node_filesystem_avail_bytes` of the server's
  metrics): uploads, backups and the journal grow.
