package metrics

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type MetricKey string

const (
	ServerRPCStatuses        MetricKey = "eigenda_proxy_http_server_requests_total"
	SecondaryRequestStatuses MetricKey = "eigenda_proxy_secondary_requests_total"
)

// MetricWithCount represents a metric with labels (key-value pairs) and a count
type MetricWithCount struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Count  int               `json:"count"`
}

func parseMetric(input string) (MetricWithCount, error) {
	// Regular expression to match the metric name, key-value pairs, and count
	re := regexp.MustCompile(`^(\w+)\{([^}]*)\}\s+(\d+)$`)
	match := re.FindStringSubmatch(input)

	if len(match) != 4 {
		return MetricWithCount{}, fmt.Errorf("invalid metric format")
	}

	// Extract the name and count
	name := match[1]
	labelsString := match[2]
	count, err := strconv.Atoi(match[3])
	if err != nil {
		return MetricWithCount{}, fmt.Errorf("invalid count value: %v", err)
	}

	// Extract the labels (key-value pairs) from the second capture group
	labelsRe := regexp.MustCompile(`(\w+)="([^"]+)"`)
	labelsMatches := labelsRe.FindAllStringSubmatch(labelsString, -1)

	labels := make(map[string]string)
	for _, labelMatch := range labelsMatches {
		key := labelMatch[1]
		value := labelMatch[2]
		labels[key] = value
	}

	// Return the parsed metric with labels and count
	return MetricWithCount{
		Name:   name,
		Labels: labels,
		Count:  count,
	}, nil
}

// PollerClient ... used to poll metrics from server
// used in E2E testing to assert client->server interactions
type PollerClient struct {
	address string
	client  *http.Client
}

func NewPoller(address string) *PollerClient {
	return &PollerClient{
		address: address,
		client:  &http.Client{},
	}
}

func BuildSecondaryCountLabels(backendType, method, status string) map[string]string {
	return map[string]string{
		"backend_type": backendType,
		"method":       method,
		"status":       status,
	}
}

// "method", "status", "commitment_mode", "cert_version"
func BuildServerRPCLabels(method, status, commitmentMode, certVersion string) map[string]string {
	return map[string]string{
		"method":          method,
		"status":          status,
		"commitment_mode": commitmentMode,
		"cert_version":    certVersion,
	}

}

type MetricSlice []*MetricWithCount

func hasMetric(line string, labels map[string]string) bool {
	for label, value := range labels {
		if !strings.Contains(line, label) {
			return false
		}

		if !strings.Contains(line, value) {
			return false
		}
	}

	return true
}

// PollMetricsWithRetry ... Polls for a Count Metric using a simple retry strategy of 1 second sleep x times
// keeping this non-modular is ok since this is only used for testing
func (m *PollerClient) PollMetricsWithRetry(name MetricKey, labels map[string]string, times int) (MetricSlice, error) {
	var ms MetricSlice
	var err error

	for i := 0; i < times; i++ {
		ms, err = m.PollMetrics(name, labels)
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}

		return ms, nil

	}

	return nil, err
}

// PollMetrics ... polls metrics from the given address and does a linear search
// provided the metric name
// assumes 1 metric to key mapping
func (m *PollerClient) PollMetrics(name MetricKey, labels map[string]string) (MetricSlice, error) {
	str, err := m.fetchMetrics(m.address)
	if err != nil {
		return nil, err
	}

	entries := []*MetricWithCount{}

	lines := strings.Split(str, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, string(name)) && hasMetric(line, labels) {
			mc, err := parseMetric(line)
			if err != nil {
				return nil, err
			}

			entries = append(entries, &mc)
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no entries found for metric: %s", name)
	}

	return entries, nil

}

// fetchMetrics ... reads metrics server endpoint contents into string
func (m *PollerClient) fetchMetrics(address string) (string, error) {
	resp, err := m.client.Get(address)
	if err != nil {
		return "", fmt.Errorf("error polling metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(body), nil
}
