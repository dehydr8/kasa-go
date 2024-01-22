package model

import "crypto/rsa"

type Credentials struct {
	Username string
	Password string
}

type DeviceConfig struct {
	Address string

	Credentials     *Credentials
	CredentialsHash *string

	Key *rsa.PrivateKey
}
