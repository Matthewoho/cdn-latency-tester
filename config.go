package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ===============================
// 配置加载模块
// ===============================

// 默认配置文件路径
const defaultConfigPath = "config.yaml"

// Config 运行时配置
type Config struct {
	Domain    string        // 测试域名
	Path      string        // API路径
	TestCount int           // 每个endpoint测试次数
	Timeout   time.Duration // 请求超时
	Interval  time.Duration // 请求间隔
	Endpoints []Endpoint    // 待测试的endpoint列表

	// 输出配置
	OutputDir  string // 输出目录
	EnableLog  bool   // 是否启用日志
	EnableJSON bool   // 是否生成 JSON 报告
	EnableHTML bool   // 是否生成 HTML 报告
}

// Endpoint 端点配置
type Endpoint struct {
	IP       string   // IP地址
	Protocol Protocol // 协议类型
	Name     string   // 名称（用于显示）
}

// Protocol 协议类型
type Protocol int

const (
	HTTP1 Protocol = iota
	HTTP2
	HTTP3
)

func (p Protocol) String() string {
	switch p {
	case HTTP1:
		return "HTTP/1.1"
	case HTTP2:
		return "HTTP/2"
	case HTTP3:
		return "HTTP/3"
	default:
		return "Unknown"
	}
}

// parseProtocol 解析协议字符串
func parseProtocol(s string) Protocol {
	switch s {
	case "HTTP/3", "http3", "h3":
		return HTTP3
	case "HTTP/2", "http2", "h2":
		return HTTP2
	default:
		return HTTP1
	}
}

// ===============================
// YAML 配置结构
// ===============================

type yamlConfig struct {
	Domain    string `yaml:"domain"`
	Path      string `yaml:"path"`
	TestCount int    `yaml:"test_count"`
	Timeout   string `yaml:"timeout"`
	Interval  string `yaml:"interval"`
	Endpoints []struct {
		Name     string `yaml:"name"`
		IP       string `yaml:"ip"`
		Protocol string `yaml:"protocol"`
	} `yaml:"endpoints"`
	Output struct {
		Dir        string `yaml:"dir"`
		EnableLog  bool   `yaml:"enable_log"`
		EnableJSON bool   `yaml:"enable_json"`
		EnableHTML bool   `yaml:"enable_html"`
	} `yaml:"output"`
}

// LoadConfig 从 YAML 文件加载配置
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 解析超时时间
	timeout, err := time.ParseDuration(yc.Timeout)
	if err != nil {
		timeout = 30 * time.Second
	}

	// 解析请求间隔
	interval, err := time.ParseDuration(yc.Interval)
	if err != nil {
		interval = 100 * time.Millisecond
	}

	// 转换端点配置
	endpoints := make([]Endpoint, len(yc.Endpoints))
	for i, ep := range yc.Endpoints {
		endpoints[i] = Endpoint{
			Name:     ep.Name,
			IP:       ep.IP,
			Protocol: parseProtocol(ep.Protocol),
		}
	}

	// 设置默认值
	outputDir := yc.Output.Dir
	if outputDir == "" {
		outputDir = "./output"
	}

	return &Config{
		Domain:     yc.Domain,
		Path:       yc.Path,
		TestCount:  yc.TestCount,
		Timeout:    timeout,
		Interval:   interval,
		Endpoints:  endpoints,
		OutputDir:  outputDir,
		EnableLog:  yc.Output.EnableLog,
		EnableJSON: yc.Output.EnableJSON,
		EnableHTML: yc.Output.EnableHTML,
	}, nil
}
