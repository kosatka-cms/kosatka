# Monitoring

Files for watching Kosatka sites with Grafana; the guide is
[docs/guides/monitoring.md](../../docs/guides/monitoring.md).

| File | |
|---|---|
| `config.alloy` | Grafana Alloy on the site's server: the server's metrics, the site's `/metrics` and its log, sent to Prometheus and Loki |
| `kosatka-dashboard.json` | the Grafana dashboard for the sites: requests, answer time, routes, the database pool, the process, the log. *Dashboards*, *New*, *Import*, upload the file |
| `alerts.yml` | alert rules in the Prometheus format: a site down, server errors, slow answers, waits for a database connection, restarts, the disk filling up |

They expect the labels `config.alloy` gives: `site` on the sites'
metrics, `job="kosatka"` and `site` on their log.
