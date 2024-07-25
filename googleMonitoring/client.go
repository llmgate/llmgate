package googlemonitoring

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/option"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MonitoringClient struct {
	projectId  string
	client     *monitoring.MetricClient
	mu         sync.RWMutex
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
}

func NewMonitoringClient(ctx context.Context, projectId, jsonCredentialsStr string) (*MonitoringClient, error) {
	var client *monitoring.MetricClient
	var err error
	if jsonCredentialsStr == "" {
		// for prod where you can fetch it from gcp service account
		client, err = monitoring.NewMetricClient(ctx)
	} else {
		client, err = monitoring.NewMetricClient(ctx, option.WithCredentialsJSON([]byte(jsonCredentialsStr)))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create Monitoring client: %w", err)
	}

	return &MonitoringClient{
		projectId:  projectId,
		client:     client,
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
	}, nil
}

func (c *MonitoringClient) Close() error {
	return c.client.Close()
}

func (c *MonitoringClient) getOrCreateCounterVec(metricName string, labels []string) *prometheus.CounterVec {
	c.mu.RLock()
	counter, exists := c.counters[metricName]
	c.mu.RUnlock()
	if exists {
		return counter
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	counter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricName,
		Help: "Dynamically created counter",
	}, labels)
	prometheus.MustRegister(counter)
	c.counters[metricName] = counter
	return counter
}

func (c *MonitoringClient) getOrCreateHistogramVec(metricName string, labels []string) *prometheus.HistogramVec {
	c.mu.RLock()
	histogram, exists := c.histograms[metricName]
	c.mu.RUnlock()
	if exists {
		return histogram
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	histogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricName,
		Help:    "Dynamically created histogram",
		Buckets: prometheus.DefBuckets,
	}, labels)
	prometheus.MustRegister(histogram)
	c.histograms[metricName] = histogram
	return histogram
}

func (c *MonitoringClient) RecordCounter(metricName string, labels map[string]string, value float64) {
	labelNames, labelValues := splitLabels(labels)
	counter := c.getOrCreateCounterVec(metricName, labelNames)
	counter.WithLabelValues(labelValues...).Add(value)
}

func (c *MonitoringClient) RecordTimer(metricName string, labels map[string]string, duration time.Duration) {
	labelNames, labelValues := splitLabels(labels)
	histogram := c.getOrCreateHistogramVec(metricName, labelNames)
	histogram.WithLabelValues(labelValues...).Observe(duration.Seconds())
}

func splitLabels(labels map[string]string) ([]string, []string) {
	var names, values []string
	for name, value := range labels {
		names = append(names, name)
		values = append(values, value)
	}
	return names, values
}

func (c *MonitoringClient) PushMetrics(ctx context.Context) error {
	now := time.Now()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %w", err)
	}

	var timeSeries []*monitoringpb.TimeSeries

	for _, mf := range mfs {
		if strings.HasPrefix(*mf.Name, "go_") || strings.HasPrefix(*mf.Name, "promhttp_") {
			continue
		}

		for _, m := range mf.Metric {
			labels := make(map[string]string)
			for _, l := range m.Label {
				labels[l.GetName()] = l.GetValue()
			}

			var value float64
			switch {
			case m.Gauge != nil:
				value = m.Gauge.GetValue()
			case m.Counter != nil:
				value = m.Counter.GetValue()
			case m.Summary != nil:
				value = m.Summary.GetSampleSum()
			case m.Histogram != nil:
				value = m.Histogram.GetSampleSum()
			default:
				fmt.Printf("Unhandled metric type: %s\n", *mf.Name)
				continue
			}

			timeSeries = append(timeSeries, &monitoringpb.TimeSeries{
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/" + *mf.Name,
					Labels: labels,
				},
				Resource: &monitoredres.MonitoredResource{
					Type: "global",
					Labels: map[string]string{
						"project_id": c.projectId,
					},
				},
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							EndTime: timestamppb.New(now),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: value,
							},
						},
					},
				},
			})
		}
	}

	if len(timeSeries) == 0 {
		return fmt.Errorf("no time series created")
	}

	if err := c.client.CreateTimeSeries(ctx, &monitoringpb.CreateTimeSeriesRequest{
		Name:       fmt.Sprintf("projects/%s", c.projectId),
		TimeSeries: timeSeries,
	}); err != nil {
		return fmt.Errorf("failed to write time series data: %w", err)
	}

	return nil
}
