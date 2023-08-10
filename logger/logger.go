package logger

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
}

func New() *Logger {
	return &Logger{
		Logger: logrus.New(),
	}
}

func (l *Logger) SetLevel(level string) error {
	logrusLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("failed to parse log-level: %s", err)
	}
	l.Level = logrusLevel
	return nil
}
