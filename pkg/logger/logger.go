package logger

import (
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

// Logger 包装了go-zero的logx，提供统一的日志接口
type Logger struct {
	logger logx.Logger
}

// New 创建一个新的Logger实例，设置调用栈跳过层数
func New() *Logger {
	return &Logger{
		logger: logx.WithCallerSkip(2), // 跳过logger包装器和全局函数调用
	}
}

// Info 记录信息级别日志
func (l *Logger) Info(v ...any) {
	l.logger.Info(v...)
}

// Infof 记录格式化的信息级别日志
func (l *Logger) Infof(format string, v ...any) {
	l.logger.Infof(format, v...)
}

// Infow 记录带字段的信息级别日志
func (l *Logger) Infow(msg string, fields ...logx.LogField) {
	l.logger.Infow(msg, fields...)
}

// Error 记录错误级别日志
func (l *Logger) Error(v ...any) {
	l.logger.Error(v...)
}

// Errorf 记录格式化的错误级别日志
func (l *Logger) Errorf(format string, v ...any) {
	l.logger.Errorf(format, v...)
}

// Errorw 记录带字段的错误级别日志
func (l *Logger) Errorw(msg string, fields ...logx.LogField) {
	l.logger.Errorw(msg, fields...)
}

// Debug 记录调试级别日志
func (l *Logger) Debug(v ...any) {
	l.logger.Debug(v...)
}

// Debugf 记录格式化的调试级别日志
func (l *Logger) Debugf(format string, v ...any) {
	l.logger.Debugf(format, v...)
}

// Debugw 记录带字段的调试级别日志
func (l *Logger) Debugw(msg string, fields ...logx.LogField) {
	l.logger.Debugw(msg, fields...)
}

// WithFields 创建带字段的logger
func (l *Logger) WithFields(fields ...logx.LogField) *Logger {
	return &Logger{
		logger: l.logger.WithFields(fields...),
	}
}

// 全局logger实例
var (
	defaultLogger *Logger
	once          sync.Once
)

// Config 日志配置
type Config struct {
	ServiceName string
	Mode        string // console, file, volume
	Level       string // debug, info, error, severe
	Encoding    string // json, plain
}

// DefaultConfig 返回默认配置
func DefaultConfig(serviceName string) Config {
	return Config{
		ServiceName: serviceName,
		Mode:        "console",
		Level:       "info",
		Encoding:    "json",
	}
}

// Init 初始化日志系统
func Init(serviceName string) {
	InitWithConfig(DefaultConfig(serviceName))
}

// InitWithConfig 使用自定义配置初始化日志系统
func InitWithConfig(config Config) {
	once.Do(func() {
		// 初始化logx配置
		logx.MustSetup(logx.LogConf{
			ServiceName: config.ServiceName,
			Mode:        config.Mode,
			Level:       config.Level,
			Encoding:    config.Encoding,
		})

		// 创建默认logger实例
		defaultLogger = New()
	})
}

// Close 关闭日志系统
func Close() {
	logx.Close()
}

// 全局函数，使用默认logger
func Info(v ...any) {
	defaultLogger.Info(v...)
}

func Infof(format string, v ...any) {
	defaultLogger.Infof(format, v...)
}

func Infow(msg string, fields ...logx.LogField) {
	defaultLogger.Infow(msg, fields...)
}

func Error(v ...any) {
	defaultLogger.Error(v...)
}

func Errorf(format string, v ...any) {
	defaultLogger.Errorf(format, v...)
}

func Errorw(msg string, fields ...logx.LogField) {
	defaultLogger.Errorw(msg, fields...)
}

func Debug(v ...any) {
	defaultLogger.Debug(v...)
}

func Debugf(format string, v ...any) {
	defaultLogger.Debugf(format, v...)
}

func Debugw(msg string, fields ...logx.LogField) {
	defaultLogger.Debugw(msg, fields...)
}

func WithFields(fields ...logx.LogField) *Logger {
	return defaultLogger.WithFields(fields...)
}
