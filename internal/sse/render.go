package sse

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"kube-chaos-sim/internal/k8s"
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
	b.WriteString(`<th>Name</th><th>Namespace</th><th>Status</th>`)
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
				`<button class="btn-latency" data-on:click="@post('/api/chaos/inject-latency?podName=%s&namespace=%s&seconds=5')">+5s</button>`+
				`<button class="btn-spike" data-on:click="@post('/api/chaos/memory-spike?podName=%s&namespace=%s&megabytes=100')">Spike</button>`+
			`</td>`+
			`</tr>`,
		cssClass,
		html.EscapeString(p.Name),
		html.EscapeString(p.Namespace),
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
		html.EscapeString(p.Name),
		html.EscapeString(p.Namespace),
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
	b.WriteString(`</div>`)
	b.WriteString(`</div>`)

	return b.String()
}

func RenderPDBPanel(pdbs []k8s.PDBInfo) string {
	var b strings.Builder
	b.WriteString(`<div id="pdb-panel" class="policy-panel">`)
	b.WriteString(`<h2>Pod Disruption Budget</h2>`)

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