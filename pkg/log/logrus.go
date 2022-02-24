package log

import (
	"io"
	"sync"

	"github.com/sirupsen/logrus"
)

type internal struct {
	logger *logrus.Logger
	level  logrus.Level
	prefix string
	fields Fields
}

func newLogger(logger *logrus.Logger, level logrus.Level, prefix string, fields Fields) *internal {
	return &internal{
		logger: logger,
		level:  level,
		prefix: prefix,
		fields: fields,
	}
}

func (l *internal) Print(args ...interface{}) {
	if l.level >= logrus.InfoLevel {
		l.prepareEntry().Print(args...)
	}
}

func (l *internal) Printf(format string, args ...interface{}) {
	if l.level >= logrus.InfoLevel {
		l.prepareEntry().Printf(format, args...)
	}
}

func (l *internal) Trace(args ...interface{}) {
	if l.level >= logrus.TraceLevel {
		l.prepareEntry().Trace(args...)
	}
}

func (l *internal) Tracef(format string, args ...interface{}) {
	if l.level >= logrus.TraceLevel {
		l.prepareEntry().Tracef(format, args...)
	}
}

func (l *internal) Debug(args ...interface{}) {
	if l.level >= logrus.DebugLevel {
		l.prepareEntry().Debug(args...)
	}
}

func (l *internal) Debugf(format string, args ...interface{}) {
	if l.level >= logrus.DebugLevel {
		l.prepareEntry().Debugf(format, args...)
	}
}

func (l *internal) Info(args ...interface{}) {
	if l.level >= logrus.InfoLevel {
		l.prepareEntry().Info(args...)
	}
}

func (l *internal) Infof(format string, args ...interface{}) {
	if l.level >= logrus.InfoLevel {
		l.prepareEntry().Infof(format, args...)
	}
}

func (l *internal) Warn(args ...interface{}) {
	if l.level >= logrus.WarnLevel {
		l.prepareEntry().Warn(args...)
	}
}

func (l *internal) Warnf(format string, args ...interface{}) {
	if l.level >= logrus.WarnLevel {
		l.prepareEntry().Warnf(format, args...)
	}
}

func (l *internal) Error(args ...interface{}) {
	if l.level >= logrus.ErrorLevel {
		l.prepareEntry().Error(args...)
	}
}

func (l *internal) Errorf(format string, args ...interface{}) {
	if l.level >= logrus.ErrorLevel {
		l.prepareEntry().Errorf(format, args...)
	}
}

func (l *internal) Fatal(args ...interface{}) {
	if l.level >= logrus.FatalLevel {
		l.prepareEntry().Fatal(args...)
	}
}

func (l *internal) Fatalf(format string, args ...interface{}) {
	if l.level >= logrus.FatalLevel {
		l.prepareEntry().Fatalf(format, args...)
	}
}

func (l *internal) Panic(args ...interface{}) {
	l.prepareEntry().Panic(args...)
}

func (l *internal) Panicf(format string, args ...interface{}) {
	l.prepareEntry().Panicf(format, args...)
}

func (l *internal) WithPrefix(prefix string) Logger {
	return newLogger(l.logger, l.level, prefix, l.Fields())
}

func (l *internal) Prefix() string {
	return l.prefix
}

func (l *internal) WithFields(fields Fields) Logger {
	return newLogger(l.logger, l.level, l.Prefix(), l.Fields().WithFields(fields))
}

func (l *internal) Fields() Fields {
	return l.fields
}

func (l *internal) prepareEntry() *logrus.Entry {
	return l.logger.
		WithFields(logrus.Fields(l.Fields())).
		WithField("prefix", l.Prefix())
}

func (l *internal) SetLevel(level logrus.Level) {
	l.level = level
}

var (
	mutex   sync.Mutex
	loggers map[string]Logger
	inner   *logrus.Logger
)

func init() {
	mutex = sync.Mutex{}
	loggers = make(map[string]Logger)

	inner = logrus.New()
	inner.Out = io.Discard
	inner.ReportCaller = false
	inner.Level = logrus.TraceLevel
}

func SetHooks(hooks logrus.LevelHooks) {
	mutex.Lock()
	defer mutex.Unlock()
	inner.Hooks = hooks
}

func NewLogger(prefix string) Logger {
	mutex.Lock()
	defer mutex.Unlock()
	l, found := loggers[prefix]
	if found == false {
		l = newLogger(inner, inner.Level, prefix, nil)
		loggers[prefix] = l
	}
	return l
}
