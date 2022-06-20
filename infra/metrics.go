package infra

import (
	"dogapm/internal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"regexp"
)

const (
	TypeHttp  = "http"
	TypeGrpc  = "grpc"
	TypeMySql = "mysql"
)

type customMetricsRegistry struct {
	*prometheus.Registry
	customLabels []*io_prometheus_client.LabelPair
}

func newCustomMetricsRegistry(labels map[string]string) *customMetricsRegistry {
	c := &customMetricsRegistry{
		Registry: prometheus.NewRegistry(),
	}
	for k, v := range labels {
		c.customLabels = append(c.customLabels, &io_prometheus_client.LabelPair{
			Name:  &k,
			Value: &v,
		})
	}
	return c
}

// Gather 方法在导出目标的时候会被调用
func (c *customMetricsRegistry) Gather() ([]*io_prometheus_client.MetricFamily, error) {
	metricFamilies, err := c.Registry.Gather()
	if err != nil {
		return nil, err
	}
	for _, metricFamily := range metricFamilies {
		metrics := metricFamily.Metric
		for _, metric := range metrics {
			//每一个metric中都加入自定义的label
			metric.Label = append(metric.Label, c.customLabels...)
		}
	}
	return metricFamilies, err
}

var (
	MetricReg = newCustomMetricsRegistry(map[string]string{
		"host": internal.BuildInfo.Hostname(),
		"app":  internal.BuildInfo.AppName(),
	})

	serverHandleCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "server_handle_total",
	}, []string{"type", "method", "peer", "peer_host"})

	serverHandleHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "server_handle_seconds",
	}, []string{"type", "method", "status", "peer", "peer_host"})

	clientHandleCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "client_handle_total",
	}, []string{"type", "method", "server"})

	clientHandleHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "client_handle_seconds",
	}, []string{"type", "method", "server"})

	libraryCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "lib_handle_total",
		Help: "The total number of third party library handle",
	}, []string{"type", "method", "name", "server"})
)

func init() {
	MetricReg.MustRegister(
		serverHandleCounter,
		serverHandleHistogram,
		clientHandleCounter,
		clientHandleHistogram,
		libraryCounter,
	)
	MetricReg.MustRegister(
		collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{
				Matcher: regexp.MustCompile("/.*"),
			}),
		),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}
