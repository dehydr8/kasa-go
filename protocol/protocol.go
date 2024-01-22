package protocol

type Protocol interface {
	Send(request, response interface{}) error
	Close() error
}
