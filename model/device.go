package model

import "crypto/rsa"

type Credentials struct {
	Username       string
	Password       string
	HashedPassword string
}

type DeviceConfig struct {
	Address string

	Credentials *Credentials

	Key *rsa.PrivateKey
}
