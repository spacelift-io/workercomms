package mqtt

import (
	"fmt"

	"github.com/spacelift-io/workercomms/messages"
)

type mqttLogLevel uint

const (
	mqttLogLevelDebug mqttLogLevel = iota
	mqttLogLevelInfo
	mqttLogLevelWarn
	mqttLogLevelError
)

type mqttLogger struct {
	logger messages.Logger
	level  mqttLogLevel
}

func newMQTTLogger(logger messages.Logger, level mqttLogLevel) *mqttLogger {
	return &mqttLogger{
		logger: logger,
		level:  level,
	}
}

func (l *mqttLogger) Println(v ...interface{}) {
	message := fmt.Sprint(v...)

	switch l.level {
	case mqttLogLevelError:
		l.logger.Error(nil, message)
	case mqttLogLevelWarn:
		l.logger.Warn(message)
	case mqttLogLevelInfo:
		l.logger.Info(message)
	default:
		l.logger.Debug(message)
	}
}

func (l *mqttLogger) Printf(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)

	switch l.level {
	case mqttLogLevelError:
		l.logger.Error(nil, message)
	case mqttLogLevelWarn:
		l.logger.Warn(message)
	case mqttLogLevelInfo:
		l.logger.Info(message)
	default:
		l.logger.Debug(message)
	}
}
