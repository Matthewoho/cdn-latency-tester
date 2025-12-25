package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// ===============================
// HTTP 客户端
// ===============================

// 创建 HTTP/1.1 客户端（指定IP）
func createHTTP1Client(ip string, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 提取端口
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				port = "443"
			}
			// 强制使用指定IP
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false, // 验证证书
			// 强制使用 HTTP/1.1，不进行 HTTP/2 ALPN 协商
			NextProtos: []string{"http/1.1"},
		},
		// 禁用 HTTP/2
		ForceAttemptHTTP2:   false,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// 创建 HTTP/2 客户端（指定IP，强制使用HTTP/2）
func createHTTP2Client(ip string, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 提取端口
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				port = "443"
			}
			// 强制使用指定IP
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			// 强制使用HTTP/2的ALPN
			NextProtos: []string{"h2"},
		},
		// 强制启用HTTP/2
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// 创建 HTTP/3 客户端（指定IP）
func createHTTP3Client(ip string, timeout time.Duration) *http.Client {
	transport := &http3.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		// 自定义 Dial 函数来指定IP
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (*quic.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				port = "443"
			}
			// 使用指定IP建立QUIC连接
			targetAddr := net.JoinHostPort(ip, port)
			// 解析UDP地址
			udpAddr, err := net.ResolveUDPAddr("udp", targetAddr)
			if err != nil {
				return nil, fmt.Errorf("解析UDP地址失败: %w", err)
			}
			// 创建UDP连接
			udpConn, err := net.ListenUDP("udp", nil)
			if err != nil {
				return nil, fmt.Errorf("创建UDP连接失败: %w", err)
			}
			// 使用quic.Dial建立连接
			return quic.Dial(ctx, udpConn, udpAddr, tlsCfg, cfg)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// ===============================
// 测试逻辑
// ===============================

// 执行单次请求并测量延迟
func measureRequest(client *http.Client, url string, domain string) RequestResult {
	result := RequestResult{}

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		result.Error = fmt.Sprintf("创建请求失败: %v", err)
		return result
	}

	// 设置Host头为原始域名
	req.Host = domain
	// 模拟 Chrome User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// 使用 httptrace 测量 TTFB
	var start time.Time
	var ttfb time.Duration

	var reused bool

	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			reused = connInfo.Reused
		},
		GotFirstResponseByte: func() {
			ttfb = time.Since(start)
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	// 发送请求
	start = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("请求失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.TTFB = ttfb
	result.StatusCode = resp.StatusCode
	result.Reused = reused
	result.ActualProto = resp.Proto // 记录实际使用的协议版本

	// 提取 x-source-response-time 响应头（单位：秒）
	xResponseTimeStr := resp.Header.Get("x-source-response-time")
	if xResponseTimeStr != "" {
		// 去除可能的 "s" 后缀
		xResponseTimeStr = strings.TrimSuffix(xResponseTimeStr, "s")
		xResponseTimeStr = strings.TrimSpace(xResponseTimeStr)
		if val, err := strconv.ParseFloat(xResponseTimeStr, 64); err == nil {
			// 转换为毫秒
			result.XResponseTime = val * 1000
		}
	}

	// 计算CDN延迟 = TTFB(ms) - x-source-response-time(ms)
	ttfbMs := float64(ttfb.Microseconds()) / 1000.0
	result.CDNLatency = ttfbMs - result.XResponseTime

	return result
}
