package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
)

// ===============================
// 统计计算
// ===============================

// 计算百分位数
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

// 计算汇总统计
func calculateSummary(endpoint Endpoint, results []RequestResult) Summary {
	summary := Summary{
		EndpointName: endpoint.Name,
		Protocol:     endpoint.Protocol.String(),
		TotalTests:   len(results),
	}

	var ttfbValues []float64
	var cdnLatencyValues []float64
	var xResponseTimeSum float64
	var hasXResponseTime bool

	for _, r := range results {
		if r.Error != "" {
			summary.FailCount++
			continue
		}
		summary.SuccessCount++

		ttfbMs := float64(r.TTFB.Microseconds()) / 1000.0
		ttfbValues = append(ttfbValues, ttfbMs)
		cdnLatencyValues = append(cdnLatencyValues, r.CDNLatency)
		xResponseTimeSum += r.XResponseTime
		if r.XResponseTime > 0 {
			hasXResponseTime = true
		}
	}

	if len(ttfbValues) == 0 {
		return summary
	}

	// TTFB 统计
	var ttfbSum float64
	summary.TTFBMin = ttfbValues[0]
	summary.TTFBMax = ttfbValues[0]
	for _, v := range ttfbValues {
		ttfbSum += v
		if v < summary.TTFBMin {
			summary.TTFBMin = v
		}
		if v > summary.TTFBMax {
			summary.TTFBMax = v
		}
	}
	summary.TTFBAvg = ttfbSum / float64(len(ttfbValues))
	summary.TTFBP50 = percentile(ttfbValues, 0.50)
	summary.TTFBP90 = percentile(ttfbValues, 0.90)
	summary.TTFBP95 = percentile(ttfbValues, 0.95)
	summary.TTFBP99 = percentile(ttfbValues, 0.99)

	// CDN延迟统计
	var cdnSum float64
	summary.CDNLatencyMin = cdnLatencyValues[0]
	summary.CDNLatencyMax = cdnLatencyValues[0]
	for _, v := range cdnLatencyValues {
		cdnSum += v
		if v < summary.CDNLatencyMin {
			summary.CDNLatencyMin = v
		}
		if v > summary.CDNLatencyMax {
			summary.CDNLatencyMax = v
		}
	}
	summary.CDNLatencyAvg = cdnSum / float64(len(cdnLatencyValues))
	summary.CDNLatencyP50 = percentile(cdnLatencyValues, 0.50)
	summary.CDNLatencyP90 = percentile(cdnLatencyValues, 0.90)
	summary.CDNLatencyP95 = percentile(cdnLatencyValues, 0.95)
	summary.CDNLatencyP99 = percentile(cdnLatencyValues, 0.99)

	// x-source-response-time 平均值
	summary.XResponseTimeAvg = xResponseTimeSum / float64(len(ttfbValues))
	summary.HasCDN = hasXResponseTime

	return summary
}

// ===============================
// 输出
// ===============================

// 打印详细结果表格
func printDetailTable(endpoint Endpoint, results []RequestResult) {
	fmt.Printf("\n📊 %s (%s @ %s) 详细结果:\n", endpoint.Name, endpoint.Protocol, endpoint.IP)

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{"序号", "状态码", "连接", "TTFB(ms)", "x-source-response-time(ms)", "CDN延迟(ms)", "错误"}),
	)

	for _, r := range results {
		errStr := ""
		if r.Error != "" {
			errStr = r.Error
		}

		ttfbMs := float64(r.TTFB.Microseconds()) / 1000.0

		reusedStr := "No"
		if r.Reused {
			reusedStr = "Yes"
		}

		table.Append([]string{
			fmt.Sprintf("%d", r.Index),
			fmt.Sprintf("%d", r.StatusCode),
			reusedStr,
			fmt.Sprintf("%.2f", ttfbMs),
			fmt.Sprintf("%.2f", r.XResponseTime),
			fmt.Sprintf("%.2f", r.CDNLatency),
			errStr,
		})
	}

	table.Render()
}

// 打印汇总表格
func printSummaryTable(summaries []Summary) {
	fmt.Println("\n📈 汇总统计:")

	table := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader([]string{
			"节点", "协议", "成功/总数",
			"TTFB均值", "TTFB-P50", "TTFB-P90", "TTFB-P99", "TTFB最小", "TTFB最大",
			"CDN延迟均值", "CDN-P50", "CDN-P90", "CDN-P99",
			"服务端均值",
		}),
	)

	for _, s := range summaries {
		// 如果没有 x-source-response-time 头，CDN相关列显示 "-"
		cdnLatencyAvg := "-"
		cdnP50 := "-"
		cdnP90 := "-"
		cdnP99 := "-"
		xResponseAvg := "-"
		if s.HasCDN {
			cdnLatencyAvg = fmt.Sprintf("%.2f", s.CDNLatencyAvg)
			cdnP50 = fmt.Sprintf("%.2f", s.CDNLatencyP50)
			cdnP90 = fmt.Sprintf("%.2f", s.CDNLatencyP90)
			cdnP99 = fmt.Sprintf("%.2f", s.CDNLatencyP99)
			xResponseAvg = fmt.Sprintf("%.2f", s.XResponseTimeAvg)
		}

		table.Append([]string{
			s.EndpointName,
			s.Protocol,
			fmt.Sprintf("%d/%d", s.SuccessCount, s.TotalTests),
			fmt.Sprintf("%.2f", s.TTFBAvg),
			fmt.Sprintf("%.2f", s.TTFBP50),
			fmt.Sprintf("%.2f", s.TTFBP90),
			fmt.Sprintf("%.2f", s.TTFBP99),
			fmt.Sprintf("%.2f", s.TTFBMin),
			fmt.Sprintf("%.2f", s.TTFBMax),
			cdnLatencyAvg,
			cdnP50,
			cdnP90,
			cdnP99,
			xResponseAvg,
		})
	}

	table.Render()
	fmt.Println("\n💡 说明: 所有时间单位均为毫秒(ms)")
	fmt.Println("   - TTFB: Time To First Byte，等待服务器响应的时长")
	fmt.Println("   - CDN延迟: TTFB - x-source-response-time，即网络传输 + CDN处理时间")
	fmt.Println("   - 服务端均值: x-source-response-time 的平均值，即源站处理时间")
}
