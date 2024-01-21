package exporter

import (
	"encoding/base64"
	"fmt"

	"github.com/dehydr8/kasa-go/protocol"
	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = (*PlugExporter)(nil)

type DeviceInfoResult struct {
	DeviceId              string `json:"device_id"`
	DeviceOn              bool   `json:"device_on"`
	Model                 string `json:"model"`
	Type                  string `json:"type"`
	Alias                 string `json:"nickname"`
	Rssi                  int    `json:"rssi"`
	OnTime                int    `json:"on_time"`
	SoftwareVersion       string `json:"sw_ver"`
	HardwareVersion       string `json:"hw_ver"`
	MAC                   string `json:"mac"`
	Overheated            bool   `json:"overheated"`
	PowerProtectionStatus string `json:"power_protection_status"`
	OvercurrentStatus     string `json:"overcurrent_status"`
	SignalLevel           int    `json:"signal_level"`
	SSID                  string `json:"ssid"`
}

type EnergyUsageResult struct {
	CurrentPower int `json:"current_power"`
	MonthEnergy  int `json:"month_energy"`
	MonthRuntime int `json:"month_runtime"`
	TodayEnergy  int `json:"today_energy"`
	TodayRuntime int `json:"today_runtime"`
}

type EnergyUsageResponse struct {
	protocol.AesProtoBaseResponse
	Result EnergyUsageResult `json:"result"`
}

type DeviceInfoResponse struct {
	protocol.AesProtoBaseResponse
	Result DeviceInfoResult `json:"result"`
}

type PlugExporter struct {
	target string
	proto  protocol.Protocol

	metricsUp,
	metricsRssi,
	metricsPowerLoad *prometheus.Desc
}

func NewPlugExporter(host string, proto protocol.Protocol) (*PlugExporter, error) {
	var (
		constLabels = prometheus.Labels{}
		labelNames  = []string{}
	)

	info, err := collectDeviceInfo(proto)

	if err != nil {
		return nil, err
	}

	constLabels["id"] = info.DeviceId
	constLabels["alias"] = info.Alias
	constLabels["model"] = info.Model
	constLabels["type"] = info.Type

	e := &PlugExporter{
		target: host,
		proto:  proto,
		metricsPowerLoad: prometheus.NewDesc("kasa_power_load",
			"Current power in Milliwatts (mW)",
			labelNames, constLabels),

		metricsUp: prometheus.NewDesc("kasa_online",
			"Device online",
			nil, constLabels),

		metricsRssi: prometheus.NewDesc("kasa_rssi",
			"Wifi received signal strength indicator",
			labelNames, constLabels),
	}

	return e, nil
}

func (k *PlugExporter) Collect(ch chan<- prometheus.Metric) {

	var energyUsageResponse EnergyUsageResponse

	if err := k.proto.Send(&map[string]interface{}{
		"method": "get_energy_usage",
	}, &energyUsageResponse); err == nil && energyUsageResponse.ErrorCode == 0 {
		ch <- prometheus.MustNewConstMetric(k.metricsPowerLoad, prometheus.GaugeValue, float64(energyUsageResponse.Result.CurrentPower))
	}

	var deviceInfoResponse DeviceInfoResponse

	if err := k.proto.Send(&map[string]interface{}{
		"method": "get_device_info",
	}, &deviceInfoResponse); err == nil && deviceInfoResponse.ErrorCode == 0 {

		if deviceInfoResponse.Result.DeviceOn {
			ch <- prometheus.MustNewConstMetric(k.metricsUp, prometheus.GaugeValue, 1)
		} else {
			ch <- prometheus.MustNewConstMetric(k.metricsUp, prometheus.GaugeValue, 0)
		}

		ch <- prometheus.MustNewConstMetric(k.metricsRssi, prometheus.GaugeValue, float64(deviceInfoResponse.Result.Rssi))
	}
}

func (k *PlugExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- k.metricsPowerLoad
}

func collectDeviceInfo(proto protocol.Protocol) (*DeviceInfoResult, error) {
	var response DeviceInfoResponse
	req := map[string]interface{}{
		"method": "get_device_info",
		"params": nil,
	}

	err := proto.Send(&req, &response)

	if err != nil {
		fmt.Println(err)
	}

	if response.ErrorCode != 0 {
		return nil, fmt.Errorf("error code: %d", response.ErrorCode)
	}

	// decode nickname
	nickname, err := base64.StdEncoding.DecodeString(response.Result.Alias)

	if err == nil {
		response.Result.Alias = string(nickname)
	}

	// decode ssid
	ssid, err := base64.StdEncoding.DecodeString(response.Result.SSID)

	if err == nil {
		response.Result.SSID = string(ssid)
	}

	return &response.Result, nil
}
