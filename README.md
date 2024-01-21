# kasa-go
Exporter for Kasa Smart Plug (KP125M)

## Build
```bash
# build for current arch
make build-cli

# build for RPi or any other arm target
make build-cli-arm
```

## Usage
```
Usage of kasa-exporter:
  -address string
        address to listen on
  -password string
        password for kasa login
  -port int
        port to listen on (default 9500)
  -username string
        username for kasa login
```

## Prometheus Config

```yaml
scrape_configs:
- job_name: 'kasa'
  static_configs:
  - targets:
    # IP of the plugs
    - 192.168.0.41
  metrics_path: /scrape
  relabel_configs:
    - source_labels : [__address__]
      target_label: __param_target
    - source_labels: [__param_target]
      target_label: instance
    - target_label: __address__
      # IP of the exporter
      replacement: localhost:9500
```

## Related work
* https://github.com/python-kasa/python-kasa
* https://github.com/fffonion/tplink-plug-exporter