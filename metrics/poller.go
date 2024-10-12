package metrics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// MetricsPoller ... used to poll metrics from server
// used in E2E testing to assert client->server interactions
type MetricsPoller struct {
	address string
	client  *http.Client
}

func NewPoller(address string) *MetricsPoller {
	return &MetricsPoller{
		address: address,
		client:  &http.Client{},
	}
}

// Poll ... polls metrics from the given address and does a linear search
// provided the metric name
func (m *MetricsPoller) Poll(metricName string) (string, error) {
	str, err := m.request(m.address)
	if err != nil {
		return "", err
	}

	println("body", str)

	lines := strings.Split(str, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName) {
			return line, nil
		}
	}
	return "", fmt.Errorf("metric %s not found", metricName)

}

// PollMetrics polls the Prometheus metrics from the given address
func (m *MetricsPoller) request(address string) (string, error) {
	resp, err := m.client.Get(address)
	if err != nil {
		return "", fmt.Errorf("error polling metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(body), nil
}
