package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// 全局日志记录器
var logger *Logger

// 单次请求任务
type RequestTask struct {
	Endpoint Endpoint
	Client   *http.Client
	URL      string
	Domain   string
	Index    int
}

// 请求结果（带端点信息）
type EndpointResult struct {
	Endpoint Endpoint
	Result   RequestResult
}

// 并行执行单轮测试（所有节点同时发起请求）
func runParallelRound(tasks []RequestTask, roundNum int, totalRounds int) []EndpointResult {
	var wg sync.WaitGroup
	results := make([]EndpointResult, len(tasks))

	logger.Printf("\n🔄 第 %d/%d 轮测试 (并发 %d 个请求)...\n", roundNum, totalRounds, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t RequestTask) {
			defer wg.Done()
			result := measureRequest(t.Client, t.URL, t.Domain)
			result.Index = t.Index
			results[idx] = EndpointResult{
				Endpoint: t.Endpoint,
				Result:   result,
			}
		}(i, task)
	}

	wg.Wait()

	// 打印本轮结果
	for _, er := range results {
		if er.Result.Error != "" {
			logger.Printf("  [%s/%s] ❌ 错误: %s\n",
				er.Endpoint.Name, er.Endpoint.Protocol, er.Result.Error)
		} else {
			reusedStr := "新"
			if er.Result.Reused {
				reusedStr = "复用"
			}
			logger.Printf("  [%s/%s] ✓ TTFB: %.2fms, 服务端: %.2fms, CDN延迟: %.2fms [%s] [%s]\n",
				er.Endpoint.Name, er.Endpoint.Protocol,
				float64(er.Result.TTFB.Microseconds())/1000.0,
				er.Result.XResponseTime,
				er.Result.CDNLatency,
				reusedStr,
				er.Result.ActualProto)
		}
	}

	return results
}

// ===============================
// 主函数
// ===============================

func main() {
	var err error

	// 加载配置文件
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		fmt.Println("请确保 config.yaml 文件存在，或指定配置文件路径: ./cdn-test [config.yaml]")
		return
	}

	// 初始化日志记录器
	logger, err = NewLogger(config.OutputDir, config.EnableLog)
	if err != nil {
		fmt.Printf("❌ 初始化日志失败: %v\n", err)
		return
	}
	defer logger.Close()

	// 创建测试报告
	report := NewTestReport(logger.GetStartTime(), *config)

	logger.Println("🚀 CDN延迟测试工具 (并行模式)")
	logger.Println("==============================")
	logger.LogConfig(*config)

	url := fmt.Sprintf("https://%s%s", config.Domain, config.Path)

	// 为每个 endpoint 创建客户端
	type EndpointClient struct {
		Endpoint Endpoint
		Client   *http.Client
	}
	clients := make([]EndpointClient, 0, len(config.Endpoints))

	for _, endpoint := range config.Endpoints {
		var client *http.Client
		switch endpoint.Protocol {
		case HTTP1:
			client = createHTTP1Client(endpoint.IP, config.Timeout)
		case HTTP2:
			client = createHTTP2Client(endpoint.IP, config.Timeout)
		case HTTP3:
			client = createHTTP3Client(endpoint.IP, config.Timeout)
		default:
			logger.Error("不支持的协议: %v", endpoint.Protocol)
			continue
		}
		clients = append(clients, EndpointClient{Endpoint: endpoint, Client: client})
	}

	// 收集每个 endpoint 的所有结果
	endpointResults := make(map[string][]RequestResult)
	for _, ec := range clients {
		endpointResults[ec.Endpoint.Name+"|"+ec.Endpoint.Protocol.String()] = make([]RequestResult, 0, config.TestCount)
	}

	// 并行测试：每轮所有节点同时发起请求
	for round := 1; round <= config.TestCount; round++ {
		// 构建本轮任务
		tasks := make([]RequestTask, len(clients))
		for i, ec := range clients {
			tasks[i] = RequestTask{
				Endpoint: ec.Endpoint,
				Client:   ec.Client,
				URL:      url,
				Domain:   config.Domain,
				Index:    round,
			}
		}

		// 并行执行
		results := runParallelRound(tasks, round, config.TestCount)

		// 收集结果
		for _, er := range results {
			key := er.Endpoint.Name + "|" + er.Endpoint.Protocol.String()
			endpointResults[key] = append(endpointResults[key], er.Result)
		}

		// 轮次间隔
		if round < config.TestCount {
			time.Sleep(config.Interval)
		}
	}

	// 整理结果并生成汇总
	var allSummaries []Summary

	for _, ec := range clients {
		key := ec.Endpoint.Name + "|" + ec.Endpoint.Protocol.String()
		results := endpointResults[key]

		// 保存结果到报告 (使用带协议的名称)
		reportKey := fmt.Sprintf("%s (%s)", ec.Endpoint.Name, ec.Endpoint.Protocol)
		report.AddResults(reportKey, results)

		// 打印详细结果
		printDetailTable(ec.Endpoint, results)

		// 计算并保存汇总
		summary := calculateSummary(ec.Endpoint, results)
		allSummaries = append(allSummaries, summary)
	}

	// 打印汇总对比
	if len(allSummaries) > 0 {
		printSummaryTable(allSummaries)
	}

	// 完成报告
	report.Finalize(allSummaries)

	// 导出报告
	logger.Section("报告生成")

	if config.EnableJSON {
		jsonPath, err := ExportJSON(report, config.OutputDir)
		if err != nil {
			logger.Error("导出 JSON 报告失败: %v", err)
		} else {
			logger.Printf("📄 JSON 报告: %s\n", jsonPath)
		}
	}

	if config.EnableHTML {
		htmlPath, err := ExportHTML(report, config.OutputDir)
		if err != nil {
			logger.Error("导出 HTML 报告失败: %v", err)
		} else {
			logger.Printf("🌐 HTML 报告: %s\n", htmlPath)
		}
	}

	if logger.GetLogPath() != "" {
		logger.Printf("📝 日志文件: %s\n", logger.GetLogPath())
	}

	logger.Println("\n✅ 测试完成!")
}
