package metrics

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestNewPreinitializesDashboardSeries(t *testing.T) {
	t.Parallel()

	m := New()
	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("gather registry: %v", err)
	}

	assertMetricWithLabels(t, families, "fitness_auth_attempts_total", map[string]string{
		"method": "login",
		"result": "failure",
	})
	assertMetricWithLabels(t, families, "fitness_db_queries_total", map[string]string{
		"operation": "query",
		"table":     "none",
	})
	assertMetricWithLabels(t, families, "fitness_db_query_duration_seconds", map[string]string{
		"operation": "delete",
		"table":     "none",
	})
	assertMetricWithLabels(t, families, "fitness_coach_requests_total", map[string]string{
		"result": "success",
	})
	assertMetricWithLabels(t, families, "fitness_export_jobs_completed_total", map[string]string{
		"format": "csv",
	})
	assertMetricWithLabels(t, families, "fitness_notifications_created_total", map[string]string{
		"type": "export_ready",
	})
	assertMetricWithLabels(t, families, "fitness_worker_poll_cycles_total", map[string]string{
		"task_type": "notification",
	})
}

func assertMetricWithLabels(t *testing.T, families []*dto.MetricFamily, name string, want map[string]string) {
	t.Helper()

	for _, family := range families {
		if family.GetName() != name {
			continue
		}

		for _, metric := range family.GetMetric() {
			if hasLabels(metric, want) {
				return
			}
		}

		t.Fatalf("metric family %s did not contain labels %v", name, want)
	}

	t.Fatalf("metric family %s not found", name)
}

func hasLabels(metric *dto.Metric, want map[string]string) bool {
	if len(metric.GetLabel()) != len(want) {
		return false
	}

	for _, label := range metric.GetLabel() {
		value, ok := want[label.GetName()]
		if !ok || value != label.GetValue() {
			return false
		}
	}

	return true
}
