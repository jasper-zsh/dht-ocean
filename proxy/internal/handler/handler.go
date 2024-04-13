package handler

type Handler interface {
	Start() error
	Stop()
}
