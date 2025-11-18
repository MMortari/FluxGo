package fluxgo

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
)

type LoggerInstance = logrus.Entry

type LoggerOptions struct {
	Type        string
	Level       string
	LogFilePath string
}

func (f *FluxGo) ConfigLogger(opt LoggerOptions) *FluxGo {
	log := logrus.New()

	handleLogLevel(log, opt)
	handleLogType(log, opt)

	log.WithFields(logrus.Fields{
		"environment":     f.Env,
		"service.name":    f.Name,
		"service.version": f.Version,
	})

	f.logger = logrus.NewEntry(log)

	return f
}

func handleLogLevel(log *logrus.Logger, opt LoggerOptions) {
	if opt.Level == "" {
		opt.Level = "debug"
	}

	level, err := logrus.ParseLevel(opt.Level)
	if err != nil {
		panic("Invalid log level")
	}
	log.SetLevel(level)
}
func handleLogType(log *logrus.Logger, opt LoggerOptions) {
	switch opt.Type {
	case "file":
		if opt.LogFilePath == "" {
			panic("Log file path is required for file logger type")
		}

		logFile, err := os.OpenFile(opt.LogFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Error to open log file: ", err)
		}

		log.SetOutput(logFile)
		log.SetFormatter(&logrus.JSONFormatter{
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "@timestamp",
				logrus.FieldKeyLevel: "severity",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "function.name",
			},
		})

	case "console":
		log.SetOutput(os.Stdout)

	default:
		panic("Invalid logger type")
	}
}

func (f *FluxGo) CreateLogger(c context.Context) *LoggerInstance {
	spanFields := logrus.Fields{}

	if f.apm != nil {
		span := f.apm.GetSpanFromContext(c)

		if span.SpanContext().HasTraceID() {
			spanFields["trace.id"] = span.SpanContext().TraceID().String()
		}
		if span.SpanContext().HasSpanID() {
			spanFields["transaction.id"] = span.SpanContext().SpanID().String()
			spanFields["span.id"] = span.SpanContext().SpanID().String()
		}

		return f.logger.WithFields(spanFields)
	}

	return f.logger
}
