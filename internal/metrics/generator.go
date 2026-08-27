package metrics

import (
	"sync"
	"time"

	"kube-chaos-sim/internal/k8s"
)

// Metrics holds the current computed metrics values.
type Metrics struct {
	Latency     float64 // ms
	ErrorRate   float64 // %
	ErrorBudget float64 // %
}

// Threshold defines a threshold line on the metrics chart.
type Threshold struct {
	Value float64
	Label string
	Color string
}

// MetricsHistory holds time-series data for metrics visualization.
type MetricsHistory struct {
	Latency     []float64
	ErrorRate   []float64
	ErrorBudget []float64
	Timestamps  []time.Time
}

// Generator computes and stores metrics history.
type Generator struct {
	mu       sync.RWMutex
	history  MetricsHistory
	maxSize  int
	tickRate time.Duration
	slo      float64 // SLO percentage (e.g., 80 means 80% availability target)

	// Thresholds for each metric
	latencyThresholds     []Threshold
	errorRateThresholds   []Threshold
	errorBudgetThresholds []Threshold
}

// NewGenerator creates a new metrics generator.
func NewGenerator(maxSize int, tickRate time.Duration, slo float64) *Generator {
	return &Generator{
		history: MetricsHistory{
			Latency:     make([]float64, 0, maxSize),
			ErrorRate:   make([]float64, 0, maxSize),
			ErrorBudget: make([]float64, 0, maxSize),
			Timestamps:  make([]time.Time, 0, maxSize),
		},
		maxSize:  maxSize,
		tickRate: tickRate,
		slo:      slo,

		latencyThresholds: []Threshold{
			{Value: 200, Label: "SLO", Color: "#28a745"},
			{Value: 500, Label: "Warning", Color: "#ffc107"},
			{Value: 1000, Label: "Critical", Color: "#dc3545"},
		},
		errorRateThresholds: []Threshold{
			{Value: 1, Label: "SLO", Color: "#28a745"},
			{Value: 10, Label: "Warning", Color: "#ffc107"},
			{Value: 25, Label: "Critical", Color: "#dc3545"},
		},
		errorBudgetThresholds: []Threshold{
			{Value: 50, Label: "Healthy", Color: "#28a745"},
			{Value: 20, Label: "Warning", Color: "#ffc107"},
			{Value: 0, Label: "Critical", Color: "#dc3545"},
		},
	}
}

// SetSLO updates the SLO target percentage and recalculates historical error budget.
func (g *Generator) SetSLO(slo float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if slo < 0 {
		slo = 0
	}
	if slo > 100 {
		slo = 100
	}
	g.slo = slo
	
	// Recalculate historical error budget based on new SLO
	g.recalculateErrorBudgetLocked()
}

// recalculateErrorBudgetLocked recalculates all historical error budget values
// based on current SLO and rolling average error rate. Must be called with lock held.
func (g *Generator) recalculateErrorBudgetLocked() {
	allowableError := (100 - g.slo) / 100 * 100 // e.g., SLO 80% → 20% allowable
	
	for i := range g.history.ErrorBudget {
		// Calculate rolling average error rate up to point i
		var avgErrorRate float64
		if i > 0 {
			sum := 0.0
			for j := 0; j <= i; j++ {
				sum += g.history.ErrorRate[j]
			}
			avgErrorRate = sum / float64(i+1)
		}
		
		// Calculate error budget
		var budget float64
		if allowableError > 0 {
			budget = 100 - (avgErrorRate/allowableError)*100
		} else {
			budget = 0 // SLO 100% means no errors allowed
		}
		
		if budget < 0 {
			budget = 0
		}
		if budget > 100 {
			budget = 100
		}
		
		g.history.ErrorBudget[i] = budget
	}
}

// SLO returns the current SLO target percentage.
func (g *Generator) SLO() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.slo
}

// Compute calculates metrics based on current pod state.
func (g *Generator) Compute(pods []k8s.PodInfo) Metrics {
	if len(pods) == 0 {
		return Metrics{Latency: 0, ErrorRate: 0, ErrorBudget: 100}
	}

	var notReadyCount, restartingCount, crashLoopCount int
	for _, p := range pods {
		// Skip pods in normal lifecycle states (scaling, rolling update)
		if p.Status == "Terminating" || p.Status == "Succeeded" ||
			p.Status == "Pending" || p.Status == "ContainerCreating" {
			continue
		}
		// Skip new pods that haven't passed readiness probe yet (Running + !Ready + no restarts).
		// These are normal during scaling/rolling update — not real failures.
		if !p.Ready && p.Status == "Running" && p.RestartCount == 0 {
			continue
		}
		if !p.Ready {
			notReadyCount++
		}
		if p.Status == "Restarting" {
			restartingCount++
		}
		if p.Status == "CrashLoopBackOff" || p.Status == "Error" || p.Status == "Failed" {
			crashLoopCount++
		}
	}

	// Latency: base 50ms + penalties for unhealthy pods
	baseLatency := 50.0
	latency := baseLatency +
		float64(notReadyCount)*100 +
		float64(restartingCount)*200 +
		float64(crashLoopCount)*500

	// Error Rate: 0% when healthy, increases with unhealthy pods
	errorRate := float64(notReadyCount)*10 + float64(crashLoopCount)*25
	if errorRate > 100 {
		errorRate = 100
	}

	// Error Budget: based on SLO and rolling average error rate
	// SLO 80% means we tolerate 20% errors
	// Budget = max(0, 100 - (avgErrorRate / (100-SLO)/100) * 100)
	g.mu.RLock()
	slo := g.slo
	history := g.history.ErrorRate
	g.mu.RUnlock()

	// Calculate rolling average error rate
	var avgErrorRate float64
	if len(history) > 0 {
		sum := 0.0
		for _, r := range history {
			sum += r
		}
		avgErrorRate = sum / float64(len(history))
	}

	// Allowable error percentage based on SLO
	allowableError := (100 - slo) / 100 * 100 // e.g., SLO 80% → 20% allowable
	
	// Budget calculation
	var errorBudget float64
	if allowableError > 0 {
		errorBudget = 100 - (avgErrorRate/allowableError)*100
	} else {
		errorBudget = 0 // SLO 100% means no errors allowed
	}
	
	if errorBudget < 0 {
		errorBudget = 0
	}
	if errorBudget > 100 {
		errorBudget = 100
	}

	return Metrics{
		Latency:     latency,
		ErrorRate:   errorRate,
		ErrorBudget: errorBudget,
	}
}

// Record adds a metrics sample to history (ring buffer).
func (g *Generator) Record(m Metrics) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// If at capacity, shift everything left
	if len(g.history.Timestamps) >= g.maxSize {
		g.history.Latency = g.history.Latency[1:]
		g.history.ErrorRate = g.history.ErrorRate[1:]
		g.history.ErrorBudget = g.history.ErrorBudget[1:]
		g.history.Timestamps = g.history.Timestamps[1:]
	}

	g.history.Latency = append(g.history.Latency, m.Latency)
	g.history.ErrorRate = append(g.history.ErrorRate, m.ErrorRate)
	g.history.ErrorBudget = append(g.history.ErrorBudget, m.ErrorBudget)
	g.history.Timestamps = append(g.history.Timestamps, time.Now())
}

// Snapshot returns a copy of the current metrics history.
func (g *Generator) Snapshot() MetricsHistory {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return MetricsHistory{
		Latency:     append([]float64(nil), g.history.Latency...),
		ErrorRate:   append([]float64(nil), g.history.ErrorRate...),
		ErrorBudget: append([]float64(nil), g.history.ErrorBudget...),
		Timestamps:  append([]time.Time(nil), g.history.Timestamps...),
	}
}

// LatencyThresholds returns the threshold definitions for latency.
func (g *Generator) LatencyThresholds() []Threshold {
	return g.latencyThresholds
}

// ErrorRateThresholds returns the threshold definitions for error rate.
func (g *Generator) ErrorRateThresholds() []Threshold {
	return g.errorRateThresholds
}

// ErrorBudgetThresholds returns the threshold definitions for error budget.
func (g *Generator) ErrorBudgetThresholds() []Threshold {
	return g.errorBudgetThresholds
}
