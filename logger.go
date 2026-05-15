package fluxgo

import (
	"context"
	"fmt"
	"log/slog"
	"os"

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
	file     *os.File
}
type LoggerInstance struct {
	*slog.Logger
	ctx context.Context
}

type LoggerOptions struct {
	// Options: console, file, otel
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
				if f.Env.IsTest() {
					opt.Type = "console"
				}

				var processor sdklog.Processor

				switch opt.Type {
				case "otel":
					if o.grpcConnection != nil {
						logExporter, err := otlploggrpc.New(context.Background(), otlploggrpc.WithGRPCConn(o.grpcConnection))
						if err != nil {
							return err
						}
						processor = sdklog.NewBatchProcessor(logExporter)
					}
				case "file":
					logFile, err := os.OpenFile(opt.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
					if err != nil {
						return fmt.Errorf("open log file: %w", err)
					}
					log.file = logFile
					logExporter, err := stdoutlog.New(stdoutlog.WithWriter(logFile))
					if err != nil {
						return err
					}
					processor = sdklog.NewBatchProcessor(logExporter)
				default:
					logExporter, err := stdoutlog.New()
					if err != nil {
						return err
					}
					processor = sdklog.NewSimpleProcessor(logExporter)
				}

				logProvider := sdklog.NewLoggerProvider(
					sdklog.WithProcessor(processor),
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
				if log.file != nil {
					if err := log.file.Close(); err != nil {
						return err
					}
				}
				f.log("LOGGER", "Stopped")
				return nil
			},
		})
		return nil
	})

	return f
}

func (f *FluxGo) CreateLogger(ctx context.Context) *LoggerInstance {
	return &LoggerInstance{f.logger.Logger, ctx}
}
func (f *Logger) CreateLogger(ctx context.Context) *LoggerInstance {
	return &LoggerInstance{f.Logger, ctx}
}

func (li *LoggerInstance) Info(msg string, attrs ...slog.Attr) {
	li.LogAttrs(li.ctx, slog.LevelInfo, msg, attrs...)
}
func (li *LoggerInstance) Infof(format string, a ...any) {
	li.Log(li.ctx, slog.LevelInfo, fmt.Sprintf(format, a...))
}
func (li *LoggerInstance) Debug(msg string, attrs ...slog.Attr) {
	li.LogAttrs(li.ctx, slog.LevelDebug, msg, attrs...)
}
func (li *LoggerInstance) Debugf(format string, a ...any) {
	li.Log(li.ctx, slog.LevelDebug, fmt.Sprintf(format, a...))
}
func (li *LoggerInstance) Warn(msg string, attrs ...slog.Attr) {
	li.LogAttrs(li.ctx, slog.LevelWarn, msg, attrs...)
}
func (li *LoggerInstance) Warnf(format string, a ...any) {
	li.Log(li.ctx, slog.LevelWarn, fmt.Sprintf(format, a...))
}
func (li *LoggerInstance) Error(msg string, attrs ...slog.Attr) {
	li.LogAttrs(li.ctx, slog.LevelError, msg, attrs...)
}
func (li *LoggerInstance) Errorf(format string, a ...any) {
	li.Log(li.ctx, slog.LevelError, fmt.Errorf(format, a...).Error())
}
