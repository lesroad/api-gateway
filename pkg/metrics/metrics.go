package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type APIGatewayMetrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestSize      *prometheus.HistogramVec
	ResponseSize     *prometheus.HistogramVec
	RequestsInFlight *prometheus.GaugeVec
	RequestTimeouts  *prometheus.CounterVec
	RequestErrors    *prometheus.CounterVec
}

var (
	DefaultMetrics *APIGatewayMetrics
)

func InitMetrics() *APIGatewayMetrics {
	metrics := &APIGatewayMetrics{
		// 请求总数计数器
		// Labels: client (格式: name-version), status_code
		RequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "api_gateway",
				Name:      "requests_total",
				Help:      "Total number of requests to downstream services",
			},
			[]string{"client", "status_code"},
		),

		// 请求延迟直方图（毫秒）
		// Labels: client
		// 桶分布：10ms, 50ms, 100ms, 200ms, 500ms, 1s, 2s, 5s, 10s, 30s
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "api_gateway",
				Name:      "request_duration_milliseconds",
				Help:      "Request duration in milliseconds to downstream services",
				Buckets:   []float64{10, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000},
			},
			[]string{"client"},
		),

		// 请求大小直方图（字节）
		// Labels: client
		// 桶分布：100B, 1KB, 10KB, 100KB, 1MB, 10MB
		RequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "api_gateway",
				Name:      "request_size_bytes",
				Help:      "Request size in bytes to downstream services",
				Buckets:   []float64{100, 1024, 10240, 102400, 1048576, 10485760},
			},
			[]string{"client"},
		),

		// 响应大小直方图（字节）
		// Labels: client, status_code
		ResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "api_gateway",
				Name:      "response_size_bytes",
				Help:      "Response size in bytes from downstream services",
				Buckets:   []float64{100, 1024, 10240, 102400, 1048576, 10485760},
			},
			[]string{"client", "status_code"},
		),

		// 当前正在处理的请求数（并发数）
		// Labels: client
		RequestsInFlight: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "api_gateway",
				Name:      "requests_in_flight",
				Help:      "Current number of requests being processed",
			},
			[]string{"client"},
		),

		// 超时计数器
		// Labels: client
		RequestTimeouts: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "api_gateway",
				Name:      "request_timeouts_total",
				Help:      "Total number of timeout requests to downstream services",
			},
			[]string{"client"},
		),

		// 错误计数器
		// Labels: client, error_type
		RequestErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "api_gateway",
				Name:      "request_errors_total",
				Help:      "Total number of error requests to downstream services",
			},
			[]string{"client", "error_type"},
		),
	}

	DefaultMetrics = metrics
	return metrics
}

func GetMetrics() *APIGatewayMetrics {
	if DefaultMetrics == nil {
		return InitMetrics()
	}
	return DefaultMetrics
}
