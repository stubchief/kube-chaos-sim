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