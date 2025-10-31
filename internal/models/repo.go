package models

type DBClient interface {
	Run() error
	Handle(msg CanMessage)
	HandleChannel() error
	GetChannel() chan CanMessage
	GetName() string
}
