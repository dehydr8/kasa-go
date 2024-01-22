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
NAME
  kasa-exporter (rev 756f42e)

FLAGS
  -l, --log STRING               log level: debug, info, warn, error (default: info)
  -a, --address STRING           address to listen on
  -p, --port INT                 port to listen on (default: 9500)
      --username STRING          username for kasa login
      --password STRING          password for kasa login
      --hashed_password STRING   hashed (sha1) password for kasa login
      --max_registries INT       maximum number of registries to cache (default: 16)
```

The configuration can also be passed to the program using environment variables prefixed with `KASA_EXPORTER_`.

An example docker compose file would look like:

```yaml
services:
  kasa:
    image: okhalid/kasa-exporter:latest
    environment:
      - KASA_EXPORTER_USERNAME=your_tplink_email
      - KASA_EXPORTER_HASHED_PASSWORD=your_sha1_hashed_password
    ports:
      - 9500:9500
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