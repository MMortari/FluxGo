package fluxgo

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	cloudwatch "github.com/kdar/logrus-cloudwatchlogs"
	"github.com/sirupsen/logrus"
)

type Logger = logrus.Entry

type LoggerAwsOptions struct {
	Region     string
	KeyId      string
	SecretKey  string
	GroupName  string
	StreamName string
}
type LoggerOptions struct {
	Type        string
	Level       string
	LogFilePath string
	Aws         *LoggerAwsOptions
}

func (f *FluxGo) ConfigLogger(opt LoggerOptions) *FluxGo {
	if f.Env.IsTest() {
		opt.Type = "console"
	}

	log := logrus.New()

	handleLogLevel(log, opt)
	handleLogType(log, opt)

	f.logger = log.WithFields(logrus.Fields{
		"environment":     f.Env.Env,
		"service.name":    f.GetCleanName(),
		"service.version": f.Version,
	})

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

	case "aws":
		if opt.Aws == nil {
			panic("AWS logger options are required for AWS logger type")
		}

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(opt.Aws.Region),
			Credentials: credentials.NewStaticCredentials(opt.Aws.KeyId, opt.Aws.SecretKey, ""),
		})
		if err != nil {
			log.Fatalf("Error to create aws session: %v", err)
		}

		hook, err := cloudwatch.NewHook(opt.Aws.GroupName, opt.Aws.StreamName, sess)
		if err != nil {
			log.Fatalf("Error creating CloudWatch hook: %v", err)
		}

		log.AddHook(hook)

	default:
		panic("Invalid logger type")
	}
}

func (f *FluxGo) CreateLogger(c context.Context) *Logger {
	if f.apm != nil {
		span := f.apm.GetSpanFromContext(c)

		spanFields := logrus.Fields{}

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
