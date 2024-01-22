package logger

import (
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var (
	GLOBAL_LOGGER log.Logger
)

func SetupLogging(lvl string) {
	GLOBAL_LOGGER = log.NewLogfmtLogger(os.Stderr)
	GLOBAL_LOGGER = level.NewFilter(GLOBAL_LOGGER, level.Allow(level.ParseDefault(lvl, level.InfoValue())))
	GLOBAL_LOGGER = log.With(GLOBAL_LOGGER, "ts", log.TimestampFormat(
		func() time.Time { return time.Now() },
		time.DateTime,
	), "caller", log.Caller(4))
}

func Info(keyvals ...interface{}) error {
	return level.Info(GLOBAL_LOGGER).Log(keyvals...)
}

func Debug(keyvals ...interface{}) error {
	return level.Debug(GLOBAL_LOGGER).Log(keyvals...)
}

func Warn(keyvals ...interface{}) error {
	return level.Warn(GLOBAL_LOGGER).Log(keyvals...)
}

func Error(keyvals ...interface{}) error {
	return level.Error(GLOBAL_LOGGER).Log(keyvals...)
}
