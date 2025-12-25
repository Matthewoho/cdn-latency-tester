# CDN 延迟测试工具

一个用于对比测试不同 CDN 节点在不同协议（HTTP/1.1, HTTP/2, HTTP/3）下延迟性能的 Go 工具。

## ✨ 功能特性

- **多协议支持**: HTTP/1.1, HTTP/2, HTTP/3 (QUIC)
- **并行测试模式**: 所有节点同时发起请求，确保在相同网络环境下公平对比
- **强制 IP 测试**: 指定特定 IP 进行测试（绕过 DNS），保持 Host 头
- **动态配置**: 通过 YAML 配置文件加载，无需重新编译
- **延迟分析**:
  - TTFB (Time To First Byte) - 总延迟
  - CDN 延迟 = TTFB - 服务端响应时间
  - 服务端响应时间 (x-source-response-time)
- **丰富的统计**: 均值、最小/最大、P50/P90/P95/P99 百分位
- **可视化报告**:
  - 📊 堆叠条形图 - CDN 延迟 + 服务端响应 = TTFB（按协议分组对比）
  - 📈 折线图 - 每个端点的延迟趋势变化
  - 🎨 性能颜色编码 - 绿/黄/红表示性能档位
  - 📋 可折叠详情表格
- **多格式导出**: JSON、HTML、日志文件

## 📁 项目结构

```
cdn-latency-tester/
├── main.go       # 程序入口，并行测试主流程
├── config.go     # 配置加载模块
├── config.yaml   # 配置文件（修改此文件配置测试参数）
├── client.go     # HTTP 客户端（H1/H2/H3）和请求测量
├── model.go      # 数据结构定义
├── report.go     # 统计计算和控制台输出
├── exporter.go   # JSON/HTML 报告导出（含 Chart.js 图表）
├── logger.go     # 日志记录器
└── output/       # 生成的报告和日志
    ├── reports/  # JSON 和 HTML 报告
    └── logs/     # 测试日志
```

## 🚀 快速开始

### 1. 编译

```bash
go build -o cdn-test
```

### 2. 配置

编辑 `config.yaml` 配置测试参数：

```yaml
# 测试目标
domain: "your-domain.com"
path: "/api/health"

# 测试参数
test_count: 100
timeout: "30s"
interval: "100ms"

# CDN 节点配置
endpoints:
  - name: "CDN-A"
    ip: "1.2.3.4"
    protocol: "HTTP/3"
  
  - name: "CDN-B"
    ip: "5.6.7.8"
    protocol: "HTTP/2"
```

### 3. 运行

```bash
./cdn-test                    # 使用默认 config.yaml
./cdn-test my-config.yaml     # 指定配置文件
```

### 4. 查看报告

```bash
open output/reports/YYYY-MM-DD_HH-MM-SS.html
```

## ⚙️ 配置说明

| 配置项 | 说明 | 示例 |
|--------|------|------|
| `domain` | 测试目标域名 | `"example.com"` |
| `path` | API 路径 | `"/api/health"` |
| `test_count` | 每节点测试次数 | `100` |
| `timeout` | 请求超时时间 | `"30s"` |
| `interval` | 请求间隔 | `"100ms"` |
| `endpoints` | CDN 节点列表 | 见下方 |

### 端点配置

```yaml
endpoints:
  - name: "节点名称"      # 显示名称
    ip: "1.2.3.4"        # 节点 IP
    protocol: "HTTP/3"   # HTTP/1.1, HTTP/2, HTTP/3
```

## 📊 报告说明

### HTML 报告包含

1. **性能对比图（按协议分组）** - 堆叠条形图直观对比各节点
2. **汇总统计表** - TTFB 和 CDN 延迟的各项百分位统计
3. **详细结果（可折叠）**:
   - 📈 折线图：TTFB / CDN延迟 / 服务端响应的趋势
   - 📋 详细数据表格

### 性能颜色档位

| 颜色 | CDN 延迟 | TTFB/服务端 |
|------|----------|-------------|
| 🟢 绿色 | <50ms | <100ms |
| 💚 浅绿 | 50-150ms | 100-300ms |
| 🟡 黄色 | 150-300ms | 300-500ms |
| 🔴 红色 | >300ms | >500ms |

### 关键指标

- **TTFB**: Time To First Byte，从发起请求到收到第一个字节的总时间
- **CDN 延迟**: `TTFB - x-source-response-time`，网络传输 + CDN 处理时间
- **服务端响应**: `x-source-response-time` 头的值，源站处理时间
- **P50/P90/P99**: 排名在 50%/90%/99% 位置的延迟值

## 📦 依赖

- [quic-go](https://github.com/quic-go/quic-go) - HTTP/3 支持
- [tablewriter](https://github.com/olekukonko/tablewriter) - 控制台表格
- [yaml.v3](https://gopkg.in/yaml.v3) - YAML 配置解析
- [Chart.js](https://www.chartjs.org/) - HTML 报告图表（CDN 引入）

## 📄 License

MIT
