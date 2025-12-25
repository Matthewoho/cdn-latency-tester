package main

import "time"

// 单次请求的测量结果
type RequestResult struct {
	Index         int           // 请求序号
	TTFB          time.Duration // Time To First Byte（等待服务器响应时长）
	XResponseTime float64       // x-source-response-time 响应头的值（ms）
	CDNLatency    float64       // CDN转发延迟 = TTFB - XResponseTime（ms）
	StatusCode    int           // HTTP状态码
	Reused        bool          // 是否复用连接
	ActualProto   string        // 实际使用的协议版本（如 HTTP/1.1, HTTP/2.0）
	Error         string        // 错误信息（如果有）
}

// 汇总统计
type Summary struct {
	EndpointName string
	Protocol     string
	TotalTests   int
	SuccessCount int
	FailCount    int
	HasCDN       bool // 是否有 x-source-response-time 头（用于判断是否走CDN）

	// TTFB 统计 (ms)
	TTFBAvg float64
	TTFBMin float64
	TTFBMax float64
	TTFBP50 float64
	TTFBP90 float64
	TTFBP95 float64
	TTFBP99 float64

	// CDN延迟统计 (ms)
	CDNLatencyAvg float64
	CDNLatencyMin float64
	CDNLatencyMax float64
	CDNLatencyP50 float64
	CDNLatencyP90 float64
	CDNLatencyP95 float64
	CDNLatencyP99 float64

	// x-source-response-time 统计 (ms)
	XResponseTimeAvg float64
}
