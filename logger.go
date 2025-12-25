package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ===============================
// 日志模块
// ===============================

// Logger 日志记录器，支持同时输出到控制台和文件
type Logger struct {
	file      *os.File
	multiOut  io.Writer
	startTime time.Time
	logPath   string
}

// NewLogger 创建新的日志记录器
// 会自动创建输出目录和日志文件
func NewLogger(outputDir string, enabled bool) (*Logger, error) {
	logger := &Logger{
		startTime: time.Now(),
	}

	if !enabled {
		// 禁用日志时，只输出到控制台
		logger.multiOut = os.Stdout
		return logger, nil
	}

	// 创建日志目录
	logDir := filepath.Join(outputDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 生成日志文件名（基于时间戳）
	timestamp := logger.startTime.Format("2006-01-02_15-04-05")
	logger.logPath = filepath.Join(logDir, fmt.Sprintf("%s.log", timestamp))

	// 创建日志文件
	file, err := os.Create(logger.logPath)
	if err != nil {
		return nil, fmt.Errorf("创建日志文件失败: %w", err)
	}
	logger.file = file

	// 同时输出到控制台和文件
	logger.multiOut = io.MultiWriter(os.Stdout, file)

	return logger, nil
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// GetLogPath 获取日志文件路径
func (l *Logger) GetLogPath() string {
	return l.logPath
}

// GetStartTime 获取开始时间
func (l *Logger) GetStartTime() time.Time {
	return l.startTime
}

// Printf 格式化输出（同时写入控制台和日志文件）
func (l *Logger) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprint(l.multiOut, msg)
}

// Println 输出一行（同时写入控制台和日志文件）
func (l *Logger) Println(args ...interface{}) {
	fmt.Fprintln(l.multiOut, args...)
}

// Info 输出信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.multiOut, "[%s] INFO  %s\n", timestamp, msg)
}

// Error 输出错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.multiOut, "[%s] ERROR %s\n", timestamp, msg)
}

// Debug 输出调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.multiOut, "[%s] DEBUG %s\n", timestamp, msg)
}

// Section 输出分隔区域
func (l *Logger) Section(title string) {
	l.Println()
	l.Printf("==================== %s ====================\n", title)
}

// LogConfig 记录配置信息
func (l *Logger) LogConfig(cfg Config) {
	l.Section("测试配置")
	l.Printf("目标域名: %s\n", cfg.Domain)
	l.Printf("测试路径: %s\n", cfg.Path)
	l.Printf("每节点测试次数: %d\n", cfg.TestCount)
	l.Printf("请求超时: %s\n", cfg.Timeout)
	l.Printf("请求间隔: %s\n", cfg.Interval)
	l.Println("待测试节点:")
	for _, ep := range cfg.Endpoints {
		l.Printf("  - %s: %s (%s)\n", ep.Name, ep.IP, ep.Protocol)
	}
}

// LogRequestResult 记录单次请求结果
func (l *Logger) LogRequestResult(index, total int, result RequestResult) {
	if result.Error != "" {
		l.Printf("  [%d/%d] ❌ 错误: %s\n", index, total, result.Error)
	} else {
		reusedStr := "新连接"
		if result.Reused {
			reusedStr = "复用"
		}
		l.Printf("  [%d/%d] ✓ TTFB: %.2fms, x-source-response-time: %.2fms, CDN延迟: %.2fms [%s] [实际协议: %s]\n",
			index, total,
			float64(result.TTFB.Microseconds())/1000.0,
			result.XResponseTime,
			result.CDNLatency,
			reusedStr,
			result.ActualProto)
	}
}
