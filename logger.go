package fluxgo

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/fx"
)

type Logger struct {
	*slog.Logger
	opt LoggerOptions

	provider *sdklog.LoggerProvider
}
type LoggerInstance struct {
	*slog.Logger
	ctx context.Context
}

type LoggerOptions struct {
	Type        string
	Level       string
	LogFilePath string
}

func (f *FluxGo) ConfigLogger(opt LoggerOptions) *FluxGo {
	f.AddDependency(func() *Logger {
		log := Logger{
			Logger: otelslog.NewLogger(f.GetCleanName()).With(
				slog.String("environment", f.Env.Env),
				slog.String("service.name", f.GetCleanName()),
				slog.String("service.version", f.Version),
			),
			opt: opt,
		}
		return &log
	})
	f.AddInvoke(func(lc fx.Lifecycle, log *Logger, o *Otel) error {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				var logExporter sdklog.Exporter
				if o.grpcConnection != nil {
					logExporter, _ = otlploggrpc.New(context.Background(), otlploggrpc.WithGRPCConn(o.grpcConnection))
				} else {
					logExporter, _ = stdoutlog.New()
				}

				logProvider := sdklog.NewLoggerProvider(
					sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
					sdklog.WithResource(o.res),
				)
				global.SetLoggerProvider(logProvider)

				log.provider = logProvider

				f.log("LOGGER", "Started")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if err := log.provider.Shutdown(ctx); err != nil {
					return err
				}
				f.log("LOGGER", "Stopped")
				return nil
			},
		})
		return nil
	})

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
		return slog.NewJSONHandler(logFile, handlerOpts)

	case "console":
		return slog.NewTextHandler(os.Stdout, handlerOpts)

	default:
		panic("Invalid logger type")
	}
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

func (f *FluxGo) CreateLogger(ctx context.Context) *LoggerInstance {
	return &LoggerInstance{f.logger.Logger, ctx}
}
func (f *Logger) CreateLogger(ctx context.Context) *LoggerInstance {
	return &LoggerInstance{f.Logger, ctx}
}

func (li *LoggerInstance) Log(level slog.Level, msg string, attrs ...slog.Attr) {
	li.Logger.Log(li.ctx, level, msg, attrs)
}
func (li *LoggerInstance) Info(msg string, attrs ...slog.Attr) {
	li.Logger.Log(li.ctx, slog.LevelInfo, msg, attrs)
}
func (li *LoggerInstance) Infof(format string, a ...any) {
	li.Logger.Log(li.ctx, slog.LevelInfo, fmt.Sprintf(format, a...))
}
func (li *LoggerInstance) Debug(msg string, attrs ...slog.Attr) {
	li.Logger.Log(li.ctx, slog.LevelDebug, msg, attrs)
}
func (li *LoggerInstance) Debugf(format string, a ...any) {
	li.Logger.Log(li.ctx, slog.LevelDebug, fmt.Sprintf(format, a...))
}
func (li *LoggerInstance) Warn(msg string, attrs ...slog.Attr) {
	li.Logger.Log(li.ctx, slog.LevelWarn, msg, attrs)
}
func (li *LoggerInstance) Warnf(format string, a ...any) {
	li.Logger.Log(li.ctx, slog.LevelWarn, fmt.Sprintf(format, a...))
}
func (li *LoggerInstance) Error(msg string, attrs ...slog.Attr) {
	li.Logger.Log(li.ctx, slog.LevelError, msg, attrs)
}
func (li *LoggerInstance) Errorf(format string, a ...any) {
	li.Logger.Log(li.ctx, slog.LevelError, fmt.Sprintf(format, a...))
}
