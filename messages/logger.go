package messages

// Logger is the logging interface the transports take.
type Logger interface {
	Debug(message string)
	Info(message string)
	Warn(message string)
	Error(err error, message string)
}
