package exporter

import (
	"github.com/dehydr8/kasa-go/device"
	"github.com/dehydr8/kasa-go/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = (*PlugExporter)(nil)

type PlugExporter struct {
	device *device.Device

	metricsUp,
	metricsRssi,
	metricsPowerLoad *prometheus.Desc
}

func NewPlugExporter(device *device.Device) (*PlugExporter, error) {
	info, err := device.GetDeviceInfo()

	if err != nil {
		return nil, err
	}

	var constLabels = prometheus.Labels{
		"id":    info.DeviceId,
		"alias": info.Alias,
		"model": info.Model,
		"type":  info.Type,
	}

	e := &PlugExporter{
		device: device,
		metricsPowerLoad: prometheus.NewDesc("kasa_power_load",
			"Current power in Milliwatts (mW)",
			nil, constLabels),

		metricsUp: prometheus.NewDesc("kasa_online",
			"Device online",
			nil, constLabels),

		metricsRssi: prometheus.NewDesc("kasa_rssi",
			"Wifi received signal strength indicator",
			nil, constLabels),
	}

	return e, nil
}

func (k *PlugExporter) Collect(ch chan<- prometheus.Metric) {
	logger.Debug("msg", "collecting metrics", "target", k.device.Address())

	if energyUsage, err := k.device.GetEnergyUsage(); err == nil {
		ch <- prometheus.MustNewConstMetric(k.metricsPowerLoad, prometheus.GaugeValue, float64(energyUsage.CurrentPower))
	} else {
		logger.Warn("msg", "error getting energy usage", "err", err)
	}

	if deviceInfo, err := k.device.GetDeviceInfo(); err == nil {
		if deviceInfo.DeviceOn {
			ch <- prometheus.MustNewConstMetric(k.metricsUp, prometheus.GaugeValue, 1)
		} else {
			ch <- prometheus.MustNewConstMetric(k.metricsUp, prometheus.GaugeValue, 0)
		}

		ch <- prometheus.MustNewConstMetric(k.metricsRssi, prometheus.GaugeValue, float64(deviceInfo.Rssi))
	} else {
		logger.Warn("msg", "error getting device info", "err", err)
	}
}

func (k *PlugExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- k.metricsPowerLoad
	ch <- k.metricsUp
	ch <- k.metricsRssi
}
