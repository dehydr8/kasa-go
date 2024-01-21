package protocol

type Credentials struct {
	Username string
	Password string
}

type DeviceConfig struct {
	Address string

	Credentials     *Credentials
	CredentialsHash *string
}

type Protocol interface {
	Send(request, response interface{}) error
	Close() error
}
