package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// ===============================
// 报告导出模块
// ===============================

// TestReport 完整测试报告
type TestReport struct {
	StartTime           time.Time                  `json:"start_time"` // 测试开始时间
	EndTime             time.Time                  `json:"end_time"`   // 测试结束时间
	Duration            time.Duration              `json:"duration"`   // 总耗时
	Config              ReportConfig               `json:"config"`     // 测试配置快照
	Results             map[string][]RequestResult `json:"results"`    // 按 endpoint 分组的详细结果
	Summaries           []Summary                  `json:"summaries"`  // 汇总统计
	SummariesByProtocol map[string][]Summary       `json:"-"`          // 按协议分组（仅用于 HTML 渲染）
	Protocols           []string                   `json:"-"`          // 协议列表（保持顺序）
}

// ReportConfig 配置快照（用于报告）
type ReportConfig struct {
	Domain    string         `json:"domain"`
	Path      string         `json:"path"`
	TestCount int            `json:"test_count"`
	Endpoints []EndpointInfo `json:"endpoints"`
}

// EndpointInfo 端点信息（用于报告）
type EndpointInfo struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Protocol string `json:"protocol"`
}

// NewTestReport 创建新的测试报告
func NewTestReport(startTime time.Time, cfg Config) *TestReport {
	endpoints := make([]EndpointInfo, len(cfg.Endpoints))
	for i, ep := range cfg.Endpoints {
		endpoints[i] = EndpointInfo{
			Name:     ep.Name,
			IP:       ep.IP,
			Protocol: ep.Protocol.String(),
		}
	}

	return &TestReport{
		StartTime: startTime,
		Config: ReportConfig{
			Domain:    cfg.Domain,
			Path:      cfg.Path,
			TestCount: cfg.TestCount,
			Endpoints: endpoints,
		},
		Results:             make(map[string][]RequestResult),
		SummariesByProtocol: make(map[string][]Summary),
	}
}

// Finalize 完成报告
func (r *TestReport) Finalize(summaries []Summary) {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Summaries = summaries

	// 按协议分组
	protocolOrder := []string{"HTTP/3", "HTTP/2", "HTTP/1.1"}
	r.SummariesByProtocol = make(map[string][]Summary)
	for _, s := range summaries {
		r.SummariesByProtocol[s.Protocol] = append(r.SummariesByProtocol[s.Protocol], s)
	}
	// 只保留有数据的协议
	for _, p := range protocolOrder {
		if _, ok := r.SummariesByProtocol[p]; ok {
			r.Protocols = append(r.Protocols, p)
		}
	}
}

// AddResults 添加端点测试结果
func (r *TestReport) AddResults(endpointName string, results []RequestResult) {
	r.Results[endpointName] = results
}

// ExportJSON 导出 JSON 格式报告
func ExportJSON(report *TestReport, outputDir string) (string, error) {
	// 创建报告目录
	reportDir := filepath.Join(outputDir, "reports")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return "", fmt.Errorf("创建报告目录失败: %w", err)
	}

	// 生成文件名
	timestamp := report.StartTime.Format("2006-01-02_15-04-05")
	filePath := filepath.Join(reportDir, fmt.Sprintf("%s.json", timestamp))

	// 序列化为 JSON
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("JSON 序列化失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("写入 JSON 文件失败: %w", err)
	}

	return filePath, nil
}

// ExportHTML 导出 HTML 格式报告
func ExportHTML(report *TestReport, outputDir string) (string, error) {
	// 创建报告目录
	reportDir := filepath.Join(outputDir, "reports")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return "", fmt.Errorf("创建报告目录失败: %w", err)
	}

	// 生成文件名
	timestamp := report.StartTime.Format("2006-01-02_15-04-05")
	filePath := filepath.Join(reportDir, fmt.Sprintf("%s.html", timestamp))

	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("创建 HTML 文件失败: %w", err)
	}
	defer file.Close()

	// 解析模板并渲染
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Millisecond).String()
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"ttfbMs": func(r RequestResult) float64 {
			return float64(r.TTFB.Microseconds()) / 1000.0
		},
		// 根据 TTFB 值返回性能颜色类
		"perfClass": func(ms float64) string {
			if ms < 100 {
				return "perf-excellent"
			} else if ms < 300 {
				return "perf-good"
			} else if ms < 500 {
				return "perf-fair"
			}
			return "perf-poor"
		},
		// 根据 CDN 延迟值返回性能颜色类
		"cdnPerfClass": func(ms float64) string {
			if ms < 50 {
				return "perf-excellent"
			} else if ms < 150 {
				return "perf-good"
			} else if ms < 300 {
				return "perf-fair"
			}
			return "perf-poor"
		},
		// 生成安全的 HTML ID（替换特殊字符）
		"safeID": func(s string) string {
			re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
			return re.ReplaceAllString(s, "-")
		},
	}).Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("解析 HTML 模板失败: %w", err)
	}

	if err := tmpl.Execute(file, report); err != nil {
		return "", fmt.Errorf("渲染 HTML 模板失败: %w", err)
	}

	return filePath, nil
}

// HTML 模板
const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CDN 延迟测试报告 - {{formatTime .StartTime}}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: linear-gradient(135deg, #0f0f1a 0%, #1a1a2e 50%, #16213e 100%);
            color: #e8e8e8;
            min-height: 100vh;
            padding: 20px;
        }
        .container { max-width: 1600px; margin: 0 auto; }
        h1 {
            text-align: center;
            font-size: 2.5em;
            margin-bottom: 10px;
            background: linear-gradient(90deg, #00d4ff, #7b2fff, #ff6b6b);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
        }
        .subtitle { text-align: center; color: #888; margin-bottom: 30px; }
        .card {
            background: rgba(255, 255, 255, 0.03);
            border-radius: 16px;
            padding: 24px;
            margin-bottom: 24px;
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.08);
        }
        .card h2 {
            font-size: 1.3em;
            margin-bottom: 16px;
            color: #00d4ff;
        }
        .config-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 16px;
        }
        .config-item { padding: 12px; background: rgba(0, 0, 0, 0.3); border-radius: 8px; }
        .config-item label { display: block; font-size: 0.85em; color: #888; margin-bottom: 4px; }
        .config-item span { font-size: 1.1em; font-weight: 600; color: #fff; }
        
        /* 柱状图样式 */
        .chart-container {
            margin-top: 16px;
        }
        .chart-row {
            display: flex;
            align-items: center;
            margin-bottom: 12px;
            padding: 8px 0;
        }
        .chart-label {
            width: 200px;
            flex-shrink: 0;
            display: flex;
            flex-direction: column;
            gap: 4px;
        }
        .chart-name {
            font-weight: 600;
            color: #fff;
            font-size: 0.95em;
        }
        .chart-bar-container {
            flex: 1;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .chart-bar {
            height: 28px;
            border-radius: 4px;
            transition: width 0.6s ease-out;
            min-width: 4px;
        }
        .chart-value {
            font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
            font-size: 0.9em;
            font-weight: 600;
            color: #fff;
            white-space: nowrap;
        }
        .bar-excellent { background: linear-gradient(90deg, #10b981, #34d399); }
        .bar-good { background: linear-gradient(90deg, #22c55e, #4ade80); }
        .bar-fair { background: linear-gradient(90deg, #f59e0b, #fbbf24); }
        .bar-poor { background: linear-gradient(90deg, #ef4444, #f87171); }
        
        .chart-subtitle {
            color: #888;
            font-size: 0.85em;
            margin-bottom: 16px;
        }
        .chart-group {
            margin-bottom: 16px;
            padding-bottom: 16px;
            border-bottom: 1px solid rgba(255, 255, 255, 0.05);
        }
        .chart-group:last-child {
            border-bottom: none;
            margin-bottom: 0;
        }
        .chart-row-sub {
            margin-top: 4px;
        }
        .chart-row-sub .chart-bar {
            height: 18px;
            opacity: 0.85;
        }
        
        /* 堆叠条形图样式 */
        .protocol-section {
            margin-bottom: 24px;
        }
        .protocol-title {
            font-size: 1em;
            margin-bottom: 12px;
            padding: 6px 12px;
            border-radius: 4px;
            display: inline-block;
        }
        .protocol-title.protocol-h3 {
            background: rgba(16, 185, 129, 0.15);
            color: #34d399;
        }
        .protocol-title.protocol-h2 {
            background: rgba(59, 130, 246, 0.15);
            color: #60a5fa;
        }
        .protocol-title.protocol-h1 {
            background: rgba(245, 158, 11, 0.15);
            color: #fbbf24;
        }
        .stacked-bar {
            display: flex;
            height: 32px;
            border-radius: 4px;
            overflow: hidden;
        }
        .stacked-segment {
            display: flex;
            align-items: center;
            justify-content: center;
            min-width: 40px;
            position: relative;
        }
        .segment-value {
            font-size: 0.75em;
            font-weight: 600;
            color: #fff;
            text-shadow: 0 1px 2px rgba(0,0,0,0.5);
        }
        .bar-cdn-segment {
            background: linear-gradient(90deg, #3b82f6, #60a5fa);
        }
        .bar-server-segment {
            background: linear-gradient(90deg, #8b5cf6, #a78bfa);
        }
        .bar-ttfb-segment {
            background: linear-gradient(90deg, #6b7280, #9ca3af);
        }
        .bar-cdn-color { color: #60a5fa; }
        .bar-server-color { color: #a78bfa; }
        .legend-inline { font-weight: 600; }
        .legend-divider { color: #444; }
        
        .chart-value-cdn {
            font-size: 0.8em;
            color: #aaa;
        }
        .chart-detail-text {
            font-size: 0.75em;
            color: #666;
            font-family: 'SF Mono', 'Monaco', 'Consolas', monospace;
        }

        .chart-legend {
            display: flex;
            gap: 20px;
            justify-content: center;
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px solid rgba(255, 255, 255, 0.1);
        }
        .legend-item {
            display: flex;
            align-items: center;
            gap: 6px;
            font-size: 0.8em;
            color: #888;
        }
        .legend-color {
            width: 16px;
            height: 12px;
            border-radius: 2px;
        }
        
        .gauge-protocol {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 0.75em;
            font-weight: 600;
        }
        .protocol-h3 { background: rgba(16, 185, 129, 0.2); color: #34d399; }
        .protocol-h2 { background: rgba(59, 130, 246, 0.2); color: #60a5fa; }
        .protocol-h1 { background: rgba(245, 158, 11, 0.2); color: #fbbf24; }
        
        /* 颜色等级 */
        .perf-excellent { color: #10b981; }
        .perf-good { color: #34d399; }
        .perf-fair { color: #fbbf24; }
        .perf-poor { color: #f87171; }
        
        /* 表格样式 */
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 12px;
        }
        th, td {
            padding: 12px 8px;
            text-align: left;
            border-bottom: 1px solid rgba(255, 255, 255, 0.08);
        }
        th {
            background: rgba(0, 212, 255, 0.1);
            color: #00d4ff;
            font-weight: 600;
            font-size: 0.8em;
            text-transform: uppercase;
        }
        tr:hover { background: rgba(255, 255, 255, 0.02); }
        .success { color: #4ade80; }
        .error { color: #f87171; }
        .reused { color: #fbbf24; }
        .na { color: #666; }
        
        .summary-table td { font-family: 'SF Mono', 'Monaco', 'Consolas', monospace; font-size: 0.9em; }
        
        /* 可折叠详情 */
        .collapsible {
            background: rgba(0, 0, 0, 0.2);
            border-radius: 12px;
            margin-top: 16px;
            overflow: hidden;
        }
        .collapsible-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 16px 20px;
            cursor: pointer;
            transition: background 0.2s;
        }
        .collapsible-header:hover {
            background: rgba(255, 255, 255, 0.03);
        }
        .collapsible-header h3 {
            font-size: 1em;
            color: #7b2fff;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .collapsible-toggle {
            width: 24px;
            height: 24px;
            border-radius: 50%;
            background: rgba(123, 47, 255, 0.2);
            display: flex;
            align-items: center;
            justify-content: center;
            transition: transform 0.3s;
        }
        .collapsible-toggle::after {
            content: '▼';
            font-size: 0.7em;
            color: #7b2fff;
        }
        .collapsible.open .collapsible-toggle {
            transform: rotate(180deg);
        }
        .collapsible-content {
            max-height: 0;
            overflow: hidden;
            transition: max-height 0.3s ease-out;
        }
        .collapsible.open .collapsible-content {
            max-height: 5000px;
        }
        .collapsible-inner {
            padding: 0 20px 20px;
        }
        
        .footer {
            text-align: center;
            padding: 20px;
            color: #666;
            font-size: 0.9em;
        }
        
        /* 节点颜色 */
        .node-agc { border-left: 3px solid #f472b6; }
        .node-gcp { border-left: 3px solid #60a5fa; }
        .node-default { border-left: 3px solid #a78bfa; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 CDN 延迟测试报告</h1>
        <p class="subtitle">生成时间: {{formatTime .EndTime}} | 测试耗时: {{formatDuration .Duration}}</p>

        <div class="card">
            <h2>📋 测试配置</h2>
            <div class="config-grid">
                <div class="config-item">
                    <label>目标域名</label>
                    <span>{{.Config.Domain}}</span>
                </div>
                <div class="config-item">
                    <label>测试路径</label>
                    <span>{{.Config.Path}}</span>
                </div>
                <div class="config-item">
                    <label>每节点测试次数</label>
                    <span>{{.Config.TestCount}}</span>
                </div>
                <div class="config-item">
                    <label>测试节点数</label>
                    <span>{{len .Config.Endpoints}}</span>
                </div>
            </div>
        </div>

        <div class="card">
            <h2>📊 性能对比图（按协议分组）</h2>
            <p class="chart-subtitle">堆叠图：CDN 延迟 + 服务端响应 = TTFB 总延迟（颜色表示性能档位）</p>
            {{range $proto := .Protocols}}
            <div class="protocol-section">
                <h3 class="protocol-title {{if eq $proto "HTTP/3"}}protocol-h3{{else if eq $proto "HTTP/2"}}protocol-h2{{else}}protocol-h1{{end}}">{{$proto}}</h3>
                <div class="chart-container">
                    {{range $s := index $.SummariesByProtocol $proto}}
                    <div class="chart-group">
                        <div class="chart-row">
                            <div class="chart-label">
                                <span class="chart-name">{{$s.EndpointName}}</span>
                            </div>
                            <div class="chart-bar-container">
                                {{if $s.HasCDN}}
                                <div class="stacked-bar" style="width: calc({{$s.TTFBAvg}} * 0.05%); min-width: 100px; max-width: 75%;">
                                    <div class="stacked-segment {{if lt $s.CDNLatencyAvg 50.0}}bar-excellent{{else if lt $s.CDNLatencyAvg 150.0}}bar-good{{else if lt $s.CDNLatencyAvg 300.0}}bar-fair{{else}}bar-poor{{end}}" style="flex: {{$s.CDNLatencyAvg}};">
                                        <span class="segment-value">CDN {{printf "%.0f" $s.CDNLatencyAvg}}</span>
                                    </div>
                                    <div class="stacked-segment {{if lt $s.XResponseTimeAvg 100.0}}bar-excellent{{else if lt $s.XResponseTimeAvg 200.0}}bar-good{{else if lt $s.XResponseTimeAvg 400.0}}bar-fair{{else}}bar-poor{{end}}" style="flex: {{$s.XResponseTimeAvg}};">
                                        <span class="segment-value">服务{{printf "%.0f" $s.XResponseTimeAvg}}</span>
                                    </div>
                                </div>
                                <span class="chart-value ttfb-total">= {{printf "%.0f" $s.TTFBAvg}} ms</span>
                                {{else}}
                                <div class="stacked-bar" style="width: calc({{$s.TTFBAvg}} * 0.05%); min-width: 80px; max-width: 75%;">
                                    <div class="stacked-segment {{if lt $s.TTFBAvg 100.0}}bar-excellent{{else if lt $s.TTFBAvg 300.0}}bar-good{{else if lt $s.TTFBAvg 500.0}}bar-fair{{else}}bar-poor{{end}}" style="flex: 1;">
                                        <span class="segment-value">TTFB {{printf "%.0f" $s.TTFBAvg}}</span>
                                    </div>
                                </div>
                                <span class="chart-value na">(无CDN数据)</span>
                                {{end}}
                            </div>
                        </div>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
            <div class="chart-legend">
                <span class="legend-item">颜色表示性能：</span>
                <span class="legend-item"><span class="legend-color bar-excellent"></span>&lt;50ms 优秀</span>
                <span class="legend-item"><span class="legend-color bar-good"></span>50-150ms 良好</span>
                <span class="legend-item"><span class="legend-color bar-fair"></span>150-300ms 一般</span>
                <span class="legend-item"><span class="legend-color bar-poor"></span>&gt;300ms 较差</span>
            </div>
        </div>


        <div class="card">
            <h2>📈 汇总统计</h2>
            <table class="summary-table">
                <thead>
                    <tr>
                        <th>节点</th>
                        <th>协议</th>
                        <th>成功率</th>
                        <th>TTFB 均值</th>
                        <th>TTFB P50</th>
                        <th>TTFB P90</th>
                        <th>TTFB P95</th>
                        <th>TTFB P99</th>
                        <th>CDN 均值</th>
                        <th>CDN P50</th>
                        <th>CDN P90</th>
                        <th>CDN P95</th>
                        <th>CDN P99</th>
                        <th>服务端均值</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Summaries}}
                    <tr>
                        <td>{{.EndpointName}}</td>
                        <td><span class="gauge-protocol {{if eq .Protocol "HTTP/3"}}protocol-h3{{else if eq .Protocol "HTTP/2"}}protocol-h2{{else}}protocol-h1{{end}}">{{.Protocol}}</span></td>
                        <td class="success">{{.SuccessCount}}/{{.TotalTests}}</td>
                        <td class="{{if lt .TTFBAvg 100.0}}perf-excellent{{else if lt .TTFBAvg 300.0}}perf-good{{else if lt .TTFBAvg 500.0}}perf-fair{{else}}perf-poor{{end}}">{{printf "%.0f" .TTFBAvg}}</td>
                        <td class="{{if lt .TTFBP50 100.0}}perf-excellent{{else if lt .TTFBP50 300.0}}perf-good{{else if lt .TTFBP50 500.0}}perf-fair{{else}}perf-poor{{end}}">{{printf "%.0f" .TTFBP50}}</td>
                        <td class="{{if lt .TTFBP90 100.0}}perf-excellent{{else if lt .TTFBP90 300.0}}perf-good{{else if lt .TTFBP90 500.0}}perf-fair{{else}}perf-poor{{end}}">{{printf "%.0f" .TTFBP90}}</td>
                        <td class="{{if lt .TTFBP95 100.0}}perf-excellent{{else if lt .TTFBP95 300.0}}perf-good{{else if lt .TTFBP95 500.0}}perf-fair{{else}}perf-poor{{end}}">{{printf "%.0f" .TTFBP95}}</td>
                        <td class="{{if lt .TTFBP99 100.0}}perf-excellent{{else if lt .TTFBP99 300.0}}perf-good{{else if lt .TTFBP99 500.0}}perf-fair{{else}}perf-poor{{end}}">{{printf "%.0f" .TTFBP99}}</td>
                        <td>{{if .HasCDN}}<span class="{{cdnPerfClass .CDNLatencyAvg}}">{{printf "%.0f" .CDNLatencyAvg}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                        <td>{{if .HasCDN}}<span class="{{cdnPerfClass .CDNLatencyP50}}">{{printf "%.0f" .CDNLatencyP50}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                        <td>{{if .HasCDN}}<span class="{{cdnPerfClass .CDNLatencyP90}}">{{printf "%.0f" .CDNLatencyP90}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                        <td>{{if .HasCDN}}<span class="{{cdnPerfClass .CDNLatencyP95}}">{{printf "%.0f" .CDNLatencyP95}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                        <td>{{if .HasCDN}}<span class="{{cdnPerfClass .CDNLatencyP99}}">{{printf "%.0f" .CDNLatencyP99}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                        <td>{{if .HasCDN}}<span class="{{perfClass .XResponseTimeAvg}}">{{printf "%.0f" .XResponseTimeAvg}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        {{range $name, $results := .Results}}
        <div class="collapsible" onclick="this.classList.toggle('open')">
            <div class="collapsible-header">
                <h3>🔍 {{$name}} 详细结果 ({{len $results}} 条记录)</h3>
                <div class="collapsible-toggle"></div>
            </div>
            <div class="collapsible-content">
                <div class="collapsible-inner">
                    <div class="chart-wrapper" style="height: 300px; margin-bottom: 20px;">
                        <canvas id="chart-{{$name | safeID}}"></canvas>
                    </div>
                    <script>
                    (function() {
                        var ctx = document.getElementById('chart-{{$name | safeID}}').getContext('2d');
                        var ttfbData = [{{range $i, $r := $results}}{{if $i}},{{end}}{{printf "%.2f" (ttfbMs $r)}}{{end}}];
                        var cdnData = [{{range $i, $r := $results}}{{if $i}},{{end}}{{if gt $r.XResponseTime 0.0}}{{printf "%.2f" $r.CDNLatency}}{{else}}null{{end}}{{end}}];
                        var serverData = [{{range $i, $r := $results}}{{if $i}},{{end}}{{if gt $r.XResponseTime 0.0}}{{printf "%.2f" $r.XResponseTime}}{{else}}null{{end}}{{end}}];
                        var labels = [{{range $i, $r := $results}}{{if $i}},{{end}}{{$r.Index}}{{end}}];
                        
                        new Chart(ctx, {
                            type: 'line',
                            data: {
                                labels: labels,
                                datasets: [
                                    {
                                        label: 'TTFB (ms)',
                                        data: ttfbData,
                                        borderColor: 'rgba(0, 212, 255, 0.8)',
                                        backgroundColor: 'rgba(0, 212, 255, 0.1)',
                                        fill: false,
                                        tension: 0.1,
                                        pointRadius: 2,
                                        pointHoverRadius: 5
                                    },
                                    {
                                        label: 'CDN 延迟 (ms)',
                                        data: cdnData,
                                        borderColor: 'rgba(16, 185, 129, 0.8)',
                                        backgroundColor: 'rgba(16, 185, 129, 0.1)',
                                        fill: false,
                                        tension: 0.1,
                                        pointRadius: 2,
                                        pointHoverRadius: 5
                                    },
                                    {
                                        label: '服务端响应 (ms)',
                                        data: serverData,
                                        borderColor: 'rgba(139, 92, 246, 0.8)',
                                        backgroundColor: 'rgba(139, 92, 246, 0.1)',
                                        fill: false,
                                        tension: 0.1,
                                        pointRadius: 2,
                                        pointHoverRadius: 5
                                    }
                                ]
                            },
                            options: {
                                responsive: true,
                                maintainAspectRatio: false,
                                plugins: {
                                    legend: {
                                        labels: { color: '#e8e8e8' }
                                    }
                                },
                                scales: {
                                    x: {
                                        title: { display: true, text: '请求序号', color: '#888' },
                                        ticks: { color: '#888' },
                                        grid: { color: 'rgba(255,255,255,0.05)' }
                                    },
                                    y: {
                                        title: { display: true, text: '延迟 (ms)', color: '#888' },
                                        ticks: { color: '#888' },
                                        grid: { color: 'rgba(255,255,255,0.05)' }
                                    }
                                }
                            }
                        });
                    })();
                    </script>
                    <table>
                        <thead>
                            <tr>
                                <th>序号</th>
                                <th>状态码</th>
                                <th>协议</th>
                                <th>连接</th>
                                <th>TTFB (ms)</th>
                                <th>服务端响应 (ms)</th>
                                <th>CDN 延迟 (ms)</th>
                                <th>错误</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range $results}}
                            <tr>
                                <td>{{.Index}}</td>
                                <td>{{if eq .StatusCode 200}}<span class="success">{{.StatusCode}}</span>{{else if eq .StatusCode 0}}<span class="error">-</span>{{else}}{{.StatusCode}}{{end}}</td>
                                <td>{{.ActualProto}}</td>
                                <td>{{if .Reused}}<span class="reused">复用</span>{{else}}新建{{end}}</td>
                                <td class="{{perfClass (ttfbMs .)}}">{{printf "%.2f" (ttfbMs .)}}</td>
                                <td>{{if gt .XResponseTime 0.0}}<span class="{{perfClass .XResponseTime}}">{{printf "%.2f" .XResponseTime}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                                <td>{{if gt .XResponseTime 0.0}}<span class="{{cdnPerfClass .CDNLatency}}">{{printf "%.2f" .CDNLatency}}</span>{{else}}<span class="na">-</span>{{end}}</td>
                                <td>{{if .Error}}<span class="error">{{.Error}}</span>{{else}}-{{end}}</td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
        {{end}}

        <div class="footer">
            <p>💡 TTFB = Time To First Byte | CDN延迟 = TTFB - 服务端响应时间</p>
            <p>由 CDN Latency Tester 生成</p>
        </div>
    </div>
</body>
</html>`
