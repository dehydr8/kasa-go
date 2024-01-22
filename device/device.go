package device

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"

	"github.com/dehydr8/kasa-go/model"
	"github.com/dehydr8/kasa-go/protocol"
)

type Device struct {
	config    *model.DeviceConfig
	transport protocol.Protocol
}

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

func NewDevice(key *rsa.PrivateKey, config *model.DeviceConfig) (*Device, error) {
	transport, err := protocol.NewAesTransport(key, config)
	if err != nil {
		return nil, err
	}

	return &Device{
		config:    config,
		transport: transport,
	}, nil
}

func (d *Device) Address() string {
	return d.config.Address
}

func (d *Device) GetEnergyUsage() (*EnergyUsageResult, error) {
	var response EnergyUsageResponse
	req := map[string]interface{}{
		"method": "get_energy_usage",
	}

	err := d.transport.Send(&req, &response)

	if err != nil {
		return nil, err
	}

	if response.ErrorCode != 0 {
		return nil, fmt.Errorf("error code: %d", response.ErrorCode)
	}

	return &response.Result, nil
}

func (d *Device) GetDeviceInfo() (*DeviceInfoResult, error) {
	var response DeviceInfoResponse
	req := map[string]interface{}{
		"method": "get_device_info",
	}

	err := d.transport.Send(&req, &response)

	if err != nil {
		return nil, err
	}

	if response.ErrorCode != 0 {
		return nil, fmt.Errorf("error code: %d", response.ErrorCode)
	}

	// try decoding nickname
	nickname, err := base64.StdEncoding.DecodeString(response.Result.Alias)

	if err == nil {
		response.Result.Alias = string(nickname)
	}

	// try decoding ssid
	ssid, err := base64.StdEncoding.DecodeString(response.Result.SSID)

	if err == nil {
		response.Result.SSID = string(ssid)
	}

	return &response.Result, nil
}
