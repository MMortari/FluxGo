package fluxgo

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"go.opentelemetry.io/contrib/bridges/otelslog"
)

type Logger struct {
	*slog.Logger
	opt LoggerOptions
}

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

	localHandler := buildLocalHandler(f, opt)

	var handler slog.Handler = localHandler
	if f.apm != nil && f.apm.LogProvider != nil {
		otelHandler := otelslog.NewHandler(f.GetCleanName(),
			otelslog.WithLoggerProvider(f.apm.LogProvider))
		handler = newMultiHandler(localHandler, otelHandler)
	}

	f.logger = &Logger{
		Logger: slog.New(handler).With(
			"environment", f.Env.Env,
			"service.name", f.GetCleanName(),
			"service.version", f.Version,
		),
		opt: opt,
	}

	return f
}

func buildLocalHandler(f *FluxGo, opt LoggerOptions) slog.Handler {
	handlerOpts := &slog.HandlerOptions{Level: parseLogLevel(opt.Level)}

	switch opt.Type {
	case "file":
		if opt.LogFilePath == "" {
			panic("Log file path is required for file logger type")
		}
		logFile, err := os.OpenFile(opt.LogFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic("Error opening log file: " + err.Error())
		}
		handlerOpts.ReplaceAttr = renameFileAttrs
		return slog.NewJSONHandler(logFile, handlerOpts)

	case "console":
		return slog.NewTextHandler(os.Stdout, handlerOpts)

	case "aws":
		if opt.Aws == nil {
			panic("AWS logger options are required for AWS logger type")
		}
		w := newCloudWatchWriter(opt.Aws)
		handlerOpts.ReplaceAttr = renameFileAttrs
		return slog.NewJSONHandler(w, handlerOpts)

	default:
		panic("Invalid logger type")
	}
}

// renameFileAttrs maps slog default keys to the expected JSON field names.
func renameFileAttrs(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		a.Key = "@timestamp"
	case slog.LevelKey:
		a.Key = "severity"
	case slog.MessageKey:
		a.Key = "message"
	case slog.SourceKey:
		a.Key = "function.name"
	}
	return a
}

func parseLogLevel(level string) slog.Level {
	if level == "" {
		return slog.LevelDebug
	}
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		panic("Invalid log level: " + level)
	}
}

// multiHandler fans out log records to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) slog.Handler {
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

// cloudWatchWriter implements io.Writer by sending lines to AWS CloudWatch Logs.
type cloudWatchWriter struct {
	client     *cloudwatchlogs.CloudWatchLogs
	groupName  string
	streamName string
}

func newCloudWatchWriter(opt *LoggerAwsOptions) io.Writer {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(opt.Region),
		Credentials: credentials.NewStaticCredentials(opt.KeyId, opt.SecretKey, ""),
	})
	if err != nil {
		panic("Error creating AWS session: " + err.Error())
	}
	return &cloudWatchWriter{
		client:     cloudwatchlogs.New(sess),
		groupName:  opt.GroupName,
		streamName: opt.StreamName,
	}
}

func (w *cloudWatchWriter) Write(p []byte) (int, error) {
	msg := string(p)
	now := aws.Int64(time.Now().UnixMilli())
	_, err := w.client.PutLogEvents(&cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(w.groupName),
		LogStreamName: aws.String(w.streamName),
		LogEvents: []*cloudwatchlogs.InputLogEvent{
			{Message: aws.String(msg), Timestamp: now},
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (f *FluxGo) CreateLogger(c context.Context) *Logger {
	if f.apm != nil {
		span := f.apm.GetSpanFromContext(c)
		var args []any

		if span.SpanContext().HasTraceID() {
			traceID := span.SpanContext().TraceID().String()
			args = append(args, "trace.id", traceID)
			if f.logger.opt.Type == "aws" && len(traceID) == 32 {
				args = append(args, "aws.xray.trace_id", "1-"+traceID[:8]+"-"+traceID[8:])
			}
		}
		if span.SpanContext().HasSpanID() {
			spanID := span.SpanContext().SpanID().String()
			args = append(args, "transaction.id", spanID, "span.id", spanID)
		}

		if len(args) > 0 {
			l := *f.logger
			l.Logger = f.logger.With(args...)
			return &l
		}
	}

	return f.logger
}
