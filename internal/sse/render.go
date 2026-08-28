package sse

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"kube-chaos-sim/internal/config"
	"kube-chaos-sim/internal/k8s"
	"kube-chaos-sim/internal/metrics"
)

func RenderPodGrid(pods []k8s.PodInfo) string {
	sort.Slice(pods, func(i, j int) bool {
		if pods[i].Namespace != pods[j].Namespace {
			return pods[i].Namespace < pods[j].Namespace
		}
		return pods[i].Name < pods[j].Name
	})

	var b strings.Builder
	b.WriteString(`<div id="pod-grid">`)
	b.WriteString(`<table>`)
	b.WriteString(`<thead><tr>`)
	b.WriteString(`<th>Name</th><th>CPU</th><th>Status</th>`)
	b.WriteString(`<th>Node</th><th>Zone</th><th>IP</th>`)
	b.WriteString(`<th>Ready</th><th>Restarts</th><th>Age</th>`)
	b.WriteString(`<th>Actions</th>`)
	b.WriteString(`</tr></thead>`)
	b.WriteString(`<tbody>`)

	if len(pods) == 0 {
		b.WriteString(`<tr><td colspan="10" class="empty">No pods found</td></tr>`)
	} else {
		for _, p := range pods {
			b.WriteString(renderPodRow(p))
		}
	}

	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func renderPodRow(p k8s.PodInfo) string {
	cssClass := statusToCSSClass(p.Status)
	readyStr := "✗"
	if p.Ready {
		readyStr = "✓"
	}

	cpuDisplay := p.CPUUsage
	if cpuDisplay == "" {
		cpuDisplay = "-"
	}

	return fmt.Sprintf(
		`<tr class="%s">`+
			`<td>%s</td>`+
			`<td>%s</td>`+
			`<td><span class="badge %s">%s</span></td>`+
			`<td>%s</td>`+
			`<td>%s</td>`+
			`<td>%s</td>`+
			`<td>%s</td>`+
			`<td>%d</td>`+
			`<td>%s</td>`+
			`<td class="actions">`+
				`<button class="btn-kill" data-on:click="@post('/api/chaos/kill-pod?podName=%s&namespace=%s')">Kill</button>`+
				`<button class="btn-latency" onclick="this.disabled=true; fetch('/api/chaos/inject-latency?podName=%s&namespace=%s&seconds=%d', {method:'POST'}); setTimeout(()=>{this.disabled=false;}, 5000)">+%ds</button>`+
				`<button class="btn-spike" onclick="this.disabled=true; fetch('/api/chaos/memory-spike?podName=%s&namespace=%s&megabytes=%d', {method:'POST'}); setTimeout(()=>{this.disabled=false;}, 10000)">Spike</button>`+
				`<button class="btn-cpu" onclick="this.disabled=true; fetch('/api/chaos/cpu-stress?podName=%s&namespace=%s&duration=%d', {method:'POST'}); setTimeout(()=>{this.disabled=false;}, %d000)">CPU</button>`+
			`</td>`+
			`</tr>`,
		cssClass,
		html.EscapeString(p.Name),
		html.EscapeString(cpuDisplay),
		cssClass,
		html.EscapeString(p.Status),
		html.EscapeString(p.NodeName),
		html.EscapeString(p.Zone),
		html.EscapeString(p.PodIP),
		readyStr,
		p.RestartCount,
		formatAge(p.Age),
		html.EscapeString(p.Name),
		html.EscapeString(p.Namespace),
		html.EscapeString(p.Name),
		html.EscapeString(p.Namespace),
		config.DefaultLatencySeconds,
		config.DefaultLatencySeconds,
		html.EscapeString(p.Name),
		html.EscapeString(p.Namespace),
		config.DefaultMemoryMB,
		html.EscapeString(p.Name),
		html.EscapeString(p.Namespace),
		config.DefaultCPUDuration,
		config.DefaultCPUDuration,
	)
}

func statusToCSSClass(status string) string {
	switch status {
	case "Running":
		return "status-running"
	case "Pending", "ContainerCreating", "PodInitializing":
		return "status-pending"
	case "Terminating":
		return "status-terminating"
	case "Restarting":
		return "status-restarting"
	case "Failed", "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "Error":
		return "status-error"
	case "Succeeded":
		return "status-succeeded"
	default:
		return "status-unknown"
	}
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func RenderHPAPanel(hpas []k8s.HPAInfo) string {
	var b strings.Builder
	b.WriteString(`<div id="hpa-panel" class="policy-panel">`)
	b.WriteString(`<h2>Horizontal Pod Autoscaler</h2>`)

	if len(hpas) == 0 {
		b.WriteString(`<p class="empty">No HPA found</p>`)
	} else {
		for _, h := range hpas {
			b.WriteString(renderHPACard(h))
		}
	}

	b.WriteString(`</div>`)
	return b.String()
}

func renderHPACard(h k8s.HPAInfo) string {
	var b strings.Builder
	name := html.EscapeString(h.Name)
	target := html.EscapeString(h.TargetRef)
	namespace := html.EscapeString(h.Namespace)

	b.WriteString(`<div class="policy-card">`)
	b.WriteString(`<div class="policy-header">`)
	b.WriteString(fmt.Sprintf(`<h3>%s</h3>`, name))
	b.WriteString(fmt.Sprintf(`<span class="badge status-running">Target: %s</span>`, target))
	b.WriteString(`</div>`)

	b.WriteString(`<div class="policy-stats">`)
	b.WriteString(`<div class="stat">`)
	b.WriteString(`<label>Replicas:</label>`)
	b.WriteString(fmt.Sprintf(`<span>%d / %d (min/max: %d-%d)</span>`,
		h.CurrentReplicas, h.DesiredReplicas, h.MinReplicas, h.MaxReplicas))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="stat">`)
	b.WriteString(`<label>CPU Target:</label>`)
	b.WriteString(fmt.Sprintf(`<span>%d%% (current: %d%%)</span>`,
		h.TargetCPUUtilization, h.CurrentCPUUtilization))
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="policy-controls">`)
	b.WriteString(fmt.Sprintf(`<label>Min Replicas: <span id="hpa-%s-min-val">%d</span></label>`, name, h.MinReplicas))
	b.WriteString(fmt.Sprintf(`<input type="range" id="hpa-%s-min" min="1" max="%d" value="%d" `,
		name, h.MaxReplicas, h.MinReplicas))
	b.WriteString(fmt.Sprintf(`oninput="document.getElementById('hpa-%s-min-val').textContent=this.value">`, name))

	b.WriteString(fmt.Sprintf(`<label>Max Replicas: <span id="hpa-%s-max-val">%d</span></label>`, name, h.MaxReplicas))
	b.WriteString(fmt.Sprintf(`<input type="range" id="hpa-%s-max" min="%d" max="20" value="%d" `,
		name, h.MinReplicas, h.MaxReplicas))
	b.WriteString(fmt.Sprintf(`oninput="document.getElementById('hpa-%s-max-val').textContent=this.value">`, name))

	b.WriteString(fmt.Sprintf(`<label>Target CPU %%: <span id="hpa-%s-cpu-val">%d</span></label>`, name, h.TargetCPUUtilization))
	b.WriteString(fmt.Sprintf(`<input type="range" id="hpa-%s-cpu" min="10" max="90" value="%d" `,
		name, h.TargetCPUUtilization))
	b.WriteString(fmt.Sprintf(`oninput="document.getElementById('hpa-%s-cpu-val').textContent=this.value">`, name))

	b.WriteString(fmt.Sprintf(`<button class="btn-apply" data-on:click="@post('/api/chaos/set-hpa?hpaName=%s&namespace=%s&minReplicas=' + document.getElementById('hpa-%s-min').value + '&maxReplicas=' + document.getElementById('hpa-%s-max').value + '&targetCPU=' + document.getElementById('hpa-%s-cpu').value)">Apply</button>`,
		name, namespace, name, name, name))
	
	b.WriteString(`<div class="stress-controls">`)
	b.WriteString(fmt.Sprintf(`<button class="btn-cpu-stress" onclick="this.disabled=true; fetch('/api/chaos/cpu-stress-all?deployment=podinfo&namespace=%s&duration=%d', {method:'POST'}); setTimeout(()=>{this.disabled=false;}, %d000)">CPU Stress (all pods)</button>`,
		namespace, config.DefaultCPUDuration, config.DefaultCPUDuration))
	b.WriteString(`</div>`)
	
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	return b.String()
}

func RenderPDBPanel(pdbs []k8s.PDBInfo) string {
	var b strings.Builder
	b.WriteString(`<div id="pdb-panel" class="policy-panel">`)
	b.WriteString(`<h2>Pod Disruption Budget</h2>`)
	
	// Add Rolling Update button at panel level
	b.WriteString(`<div class="policy-controls">`)
	b.WriteString(`<button class="btn-rolling-update" data-on:click="@post('/api/chaos/rolling-update?deployment=podinfo&namespace=default')">Rolling Update (podinfo)</button>`)
	b.WriteString(`</div>`)

	if len(pdbs) == 0 {
		b.WriteString(`<p class="empty">No PDB found</p>`)
	} else {
		for _, p := range pdbs {
			b.WriteString(renderPDBCard(p))
		}
	}

	b.WriteString(`</div>`)
	return b.String()
}

func renderPDBCard(p k8s.PDBInfo) string {
	var b strings.Builder
	name := html.EscapeString(p.Name)
	namespace := html.EscapeString(p.Namespace)

	b.WriteString(`<div class="policy-card">`)
	b.WriteString(`<div class="policy-header">`)
	b.WriteString(fmt.Sprintf(`<h3>%s</h3>`, name))
	b.WriteString(fmt.Sprintf(`<span class="badge status-running">Namespace: %s</span>`, namespace))
	b.WriteString(`</div>`)

	b.WriteString(`<div class="policy-stats">`)
	b.WriteString(`<div class="stat">`)
	b.WriteString(`<label>Healthy:</label>`)
	b.WriteString(fmt.Sprintf(`<span>%d / %d (current/desired)</span>`, p.CurrentHealthy, p.DesiredHealthy))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="stat">`)
	b.WriteString(`<label>Disruptions Allowed:</label>`)
	b.WriteString(fmt.Sprintf(`<span>%d</span>`, p.DisruptionsAllowed))
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	b.WriteString(`<div class="policy-controls">`)
	b.WriteString(fmt.Sprintf(`<label>Min Available: <span id="pdb-%s-min-val">%d</span></label>`, name, p.MinAvailable))
	b.WriteString(fmt.Sprintf(`<input type="range" id="pdb-%s-min" min="0" max="10" value="%d" `, name, p.MinAvailable))
	b.WriteString(fmt.Sprintf(`oninput="document.getElementById('pdb-%s-min-val').textContent=this.value">`, name))
	b.WriteString(fmt.Sprintf(`<button class="btn-apply" data-on:click="@post('/api/chaos/set-pdb?pdbName=%s&namespace=%s&minAvailable=' + document.getElementById('pdb-%s-min').value)">Apply</button>`,
		name, namespace, name))
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	return b.String()
}

// RenderMetricsPanel renders the metrics panel with three SVG charts.
func RenderMetricsPanel(history metrics.MetricsHistory, gen *metrics.Generator) string {
	var b strings.Builder
	b.WriteString(`<div id="metrics-panel">`)
	b.WriteString(`<h2>Metrics</h2>`)
	
	// Show window and SLO info with editable SLO
	slo := gen.SLO()
	b.WriteString(`<div class="metrics-info">`)
	b.WriteString(`<span class="metrics-badge">Window: 2 minutes (60 points @ 2s)</span>`)
	b.WriteString(`<span class="metrics-badge">`)
	b.WriteString(fmt.Sprintf(`SLO: <input type="number" id="slo-input" value="%.0f" min="0" max="100" step="1" style="width: 50px; padding: 2px 4px; border: 1px solid #ccc; border-radius: 3px;">%%`, slo))
	b.WriteString(fmt.Sprintf(` <button class="btn-apply-small" data-on:click="@post('/api/chaos/set-slo?slo=' + document.getElementById('slo-input').value)">Apply</button>`))
	b.WriteString(`</span>`)
	b.WriteString(`</div>`)
	
	b.WriteString(`<div class="metrics-grid">`)

	if len(history.Timestamps) == 0 {
		b.WriteString(`<p class="empty">No metrics data yet</p>`)
	} else {
		// Render three charts
		b.WriteString(renderChart("Latency (ms)", history.Latency, history.Timestamps,
			gen.LatencyThresholds(), 0, getMax(history.Latency)*1.2, "ms"))
		b.WriteString(renderChart("Error Rate (%)", history.ErrorRate, history.Timestamps,
			gen.ErrorRateThresholds(), 0, 100, "%"))
		b.WriteString(renderChart("Error Budget (%)", history.ErrorBudget, history.Timestamps,
			gen.ErrorBudgetThresholds(), 0, 100, "%"))
	}

	b.WriteString(`</div>`)
	b.WriteString(`</div>`)
	return b.String()
}

func getMax(values []float64) float64 {
	if len(values) == 0 {
		return 100
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func renderChart(title string, values []float64, timestamps []time.Time,
	thresholds []metrics.Threshold, minY, maxY float64, unit string) string {
	var b strings.Builder

	// SVG dimensions
	const width = 400
	const height = 200
	const padding = 40
	const chartWidth = width - 2*padding
	const chartHeight = height - 2*padding

	// Current value
	currentValue := values[len(values)-1]

	// Determine color based on thresholds
	lineColor := getLineColor(currentValue, thresholds, title)

	b.WriteString(`<div class="metric-chart">`)
	b.WriteString(fmt.Sprintf(`<h3>%s: <span style="color: %s">%.1f%s</span></h3>`, title, lineColor, currentValue, unit))

	b.WriteString(fmt.Sprintf(`<svg viewBox="0 0 %d %d" class="chart-svg">`, width, height))

	// Background
	b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="#f8f9fa" stroke="#e0e0e0"/>`,
		padding, padding, chartWidth, chartHeight))

	// Color zones based on thresholds
	inverted := strings.Contains(title, "Budget")
	renderColorZones(&b, thresholds, padding, chartWidth, chartHeight, minY, maxY, inverted)

	// Threshold lines
	for _, t := range thresholds {
		if t.Value >= minY && t.Value <= maxY {
			y := valueToY(t.Value, minY, maxY, padding, chartHeight)
			b.WriteString(fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2" stroke-dasharray="5,5"/>`,
				padding, y, padding+chartWidth, y, t.Color))
			b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="10" fill="%s">%s (%.0f)</text>`,
				padding+chartWidth+5, y+3, t.Color, t.Label, t.Value))
		}
	}

	// Data line
	if len(values) > 0 {
		points := make([]string, 0, len(values))
		for i, v := range values {
			var x int
			if len(values) == 1 {
				x = padding
			} else {
				x = padding + (i*chartWidth)/(len(values)-1)
			}
			y := valueToY(v, minY, maxY, padding, chartHeight)
			points = append(points, fmt.Sprintf("%.1f,%.1f", float64(x), float64(y)))
		}
		b.WriteString(fmt.Sprintf(`<polyline points="%s" fill="none" stroke="%s" stroke-width="2"/>`,
			strings.Join(points, " "), lineColor))
	}

	// Y-axis labels
	b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="10" text-anchor="end" fill="#666">%.0f</text>`,
		padding-5, padding+10, maxY))
	b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="10" text-anchor="end" fill="#666">%.0f</text>`,
		padding-5, padding+chartHeight, minY))

	// X-axis labels (time)
	if len(timestamps) > 0 {
		startTime := timestamps[0]
		endTime := timestamps[len(timestamps)-1]
		duration := endTime.Sub(startTime)

		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="10" fill="#666">%s</text>`,
			padding, padding+chartHeight+15, formatTime(startTime)))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="10" text-anchor="end" fill="#666">%s</text>`,
			padding+chartWidth, padding+chartHeight+15, formatTime(endTime)))

		// Duration label
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-size="10" text-anchor="middle" fill="#999">%s</text>`,
			padding+chartWidth/2, padding+chartHeight+15, formatDuration(duration)))
	}

	b.WriteString(`</svg>`)
	b.WriteString(`</div>`)

	return b.String()
}

func valueToY(value, minY, maxY float64, padding, chartHeight int) int {
	if maxY == minY {
		return padding + chartHeight/2
	}
	ratio := (value - minY) / (maxY - minY)
	return padding + chartHeight - int(ratio*float64(chartHeight))
}

func getLineColor(value float64, thresholds []metrics.Threshold, title string) string {
	// For error budget, higher is better (inverse logic)
	// Thresholds: Healthy=50 (green), Warning=20 (yellow), Critical=0 (red)
	if strings.Contains(title, "Budget") {
		if value >= thresholds[0].Value {
			return thresholds[0].Color // green (>=50)
		}
		if value >= thresholds[1].Value {
			return thresholds[1].Color // yellow (>=20)
		}
		return thresholds[2].Color // red (<20)
	}

	// For latency and error rate, higher is worse
	for _, t := range thresholds {
		if value < t.Value {
			return t.Color
		}
	}
	return "#dc3545"
}

func renderColorZones(b *strings.Builder, thresholds []metrics.Threshold,
	padding, chartWidth, chartHeight int, minY, maxY float64, inverted bool) {
	// Sort thresholds by value
	sorted := make([]metrics.Threshold, len(thresholds))
	copy(sorted, thresholds)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value < sorted[j].Value
	})

	// Render zones between thresholds
	prevValue := minY
	for _, t := range sorted {
		if t.Value > prevValue && t.Value <= maxY {
			y1 := valueToY(t.Value, minY, maxY, padding, chartHeight)
			y2 := valueToY(prevValue, minY, maxY, padding, chartHeight)
			b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" fill-opacity="0.1"/>`,
				padding, y1, chartWidth, y2-y1, t.Color))
			prevValue = t.Value
		}
	}

	// Last zone to maxY
	if prevValue < maxY {
		y1 := valueToY(maxY, minY, maxY, padding, chartHeight)
		y2 := valueToY(prevValue, minY, maxY, padding, chartHeight)
		// For inverted metrics (error budget), higher is better → use last threshold color (green)
		// For normal metrics (latency, error rate), higher is worse → use red
		lastColor := "#dc3545"
		if inverted {
			lastColor = sorted[len(sorted)-1].Color
		}
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" fill-opacity="0.1"/>`,
			padding, y1, chartWidth, y2-y1, lastColor))
	}
}

func formatTime(t time.Time) string {
	return t.Format("15:04:05")
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}