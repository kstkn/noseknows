# Prometheus exporter for ePIN pollen data

See https://epin.lgl.bayern.de/pollenflug-verlauf

# Usage

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'noseknows'
    scrape_interval: 3h
    static_configs:
      - targets:
        - 'localhost:9092'
```
