// Package status collects system status information from remote servers
// via a single batched SSH command, mirroring the original TypeScript
// status-collector implementation.
package status

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/overklassniy/ssh-mcp/internal/policy"
)

// ServerStatus holds the collected system status of a remote server.
type ServerStatus struct {
	Reachable         bool     `json:"reachable"`
	LastUpdated       string   `json:"lastUpdated"`
	Hostname          string   `json:"hostname,omitempty"`
	IPAddresses       []string `json:"ipAddresses,omitempty"`
	OSName            string   `json:"osName,omitempty"`
	OSVersion         string   `json:"osVersion,omitempty"`
	KernelVersion     string   `json:"kernelVersion,omitempty"`
	Uptime            string   `json:"uptime,omitempty"`
	DiskSpace         string   `json:"diskSpace,omitempty"`
	Memory            string   `json:"memory,omitempty"`
	CPUName           string   `json:"cpuName,omitempty"`
	CPUUsage          string   `json:"cpuUsage,omitempty"`
	GPUs              []string `json:"gpus,omitempty"`
	GPUPaths          []string `json:"gpuPaths,omitempty"`
	Drives            []string `json:"drives,omitempty"`
	Processes         string   `json:"processes,omitempty"`
	Threads           string   `json:"threads,omitempty"`
	ServicesRunning   string   `json:"servicesRunning,omitempty"`
	ServicesInstalled string   `json:"servicesInstalled,omitempty"`
}

// CommandRunner runs a command on the named server and returns its stdout.
type CommandRunner func(ctx context.Context, command, connectionName string) (string, error)

// CollectSystemStatus runs a batched status probe on the remote server.
// Each probe is validated against the policy before being included in the
// batch script. The runner executes the combined script and the output is
// parsed into a ServerStatus struct.
func CollectSystemStatus(ctx context.Context, run CommandRunner, connectionName string, pol *policy.Policy) (*ServerStatus, error) {
	status := &ServerStatus{
		Reachable:   true,
		LastUpdated: time.Now().UTC().Format(time.RFC3339),
	}

	commands := buildProbeCommands()

	// Filter commands through the policy
	allowed := make(map[string]string)
	for field, cmd := range commands {
		if pol != nil {
			if ok, _ := pol.Validate(cmd); !ok {
				continue
			}
		}
		allowed[field] = cmd
	}

	if len(allowed) == 0 {
		return status, nil
	}

	marker := fmt.Sprintf("__MCP_FIELD_%x_", rand.Int63n(0xffffff))
	script := buildStatusScript(allowed, marker)

	output, err := run(ctx, script, connectionName)
	if err != nil {
		// A failed command leaves every field unset, matching the original behavior.
		return status, nil
	}

	values := parseStatusScriptOutput(output, marker)

	status.Hostname = values["hostname"]
	if ipStr := values["ipAddresses"]; ipStr != "" {
		for _, ip := range strings.Split(ipStr, "\n") {
			ip = strings.TrimSpace(ip)
			if ip != "" && !strings.Contains(ip, "127.0.0.1") {
				status.IPAddresses = append(status.IPAddresses, ip)
			}
		}
	}
	status.OSName = values["osName"]
	status.OSVersion = values["osVersion"]
	status.KernelVersion = values["kernelVersion"]
	status.Uptime = values["uptime"]
	status.DiskSpace = values["diskSpace"]
	status.Memory = values["memory"]
	status.CPUName = values["cpuName"]
	status.CPUUsage = values["cpuUsage"]

	if gpuStr := values["gpus"]; gpuStr != "" {
		for _, gpu := range strings.Split(gpuStr, "\n") {
			gpu = strings.TrimSpace(gpu)
			if gpu != "" {
				status.GPUs = append(status.GPUs, gpu)
			}
		}
	}

	if gpuPathsStr := values["gpuPaths"]; gpuPathsStr != "" {
		for _, p := range strings.Split(gpuPathsStr, "\n") {
			p = strings.TrimSpace(p)
			if p != "" {
				status.GPUPaths = append(status.GPUPaths, p)
			}
		}
	}

	if drivesStr := values["drives"]; drivesStr != "" {
		for _, d := range strings.Split(drivesStr, "\n") {
			d = strings.TrimSpace(d)
			if d != "" {
				status.Drives = append(status.Drives, d)
			}
		}
	}

	status.Processes = values["processes"]
	status.Threads = values["threads"]
	status.ServicesRunning = values["servicesRunning"]
	status.ServicesInstalled = values["servicesInstalled"]

	return status, nil
}

// buildProbeCommands returns the map of field names to probe commands.
func buildProbeCommands() map[string]string {
	return map[string]string{
		"hostname":          "hostname",
		"ipAddresses":       "ip -o addr show | awk '{print $4}' | grep -v '^127\\.' | cut -d'/' -f1",
		"osName":            "uname -s",
		"osVersion":         "cat /etc/os-release 2>/dev/null | grep '^PRETTY_NAME=' | cut -d'=' -f2 | tr -d '\"' || uname -o",
		"kernelVersion":     "uname -r",
		"uptime":            "uptime -p 2>/dev/null || uptime | awk -F'up ' '{print $2}' | awk -F',' '{print $1}'",
		"diskSpace":         "df -h / | tail -1 | awk '{print \"free:\" $4 \" total:\" $2}'",
		"memory":            "free -h | grep '^Mem:' | awk '{print \"free:\" $7 \" total:\" $2}'",
		"cpuName":           "sh -c '(lscpu 2>/dev/null | grep \"^Model name:\" | cut -d\":\" -f2 | xargs || cat /proc/cpuinfo 2>/dev/null | grep \"model name\" | head -1 | cut -d\":\" -f2 | xargs || echo \"$(nproc 2>/dev/null || echo '?')-core $(uname -m 2>/dev/null || echo 'unknown') processor\") || true'",
		"cpuUsage":          "top -bn1 | grep 'Cpu(s)' | sed 's/.*, *\\([0-9.]*\\)%* id.*/\\1/' | awk '{print 100 - $1}'",
		"gpus":              "sh -c '(nvidia-smi --query-gpu=name,utilization.gpu --format=csv,noheader,nounits 2>/dev/null | while IFS=\",\" read -r name usage; do echo \"NVIDIA|${name}|${usage}\"; done || lspci | grep -iE \"vga|3d|display\" | while read -r line; do gpu_name=$(echo \"$line\" | cut -d\":\" -f3 | xargs); echo \"OTHER|${gpu_name}|\"; done) || true'",
		"gpuPaths":          "ls -1 /dev/dri/card* 2>/dev/null | sort -V || echo ''",
		"drives":            "df -h | awk 'NR>1 && $1 !~ /^(tmpfs|devtmpfs|overlay|shfs|rootfs)$/ && $6 !~ /^(\\/dev|\\/run|\\/sys|\\/proc|\\/boot|\\/usr|\\/lib)$/ && $6 != \"\" {print $1\"|\"$2\"|\"$3\"|\"$4\"|\"$5\"|\"$6}'",
		"processes":         "ps aux | wc -l",
		"threads":           "ps -eLf | wc -l",
		"servicesRunning":   "systemctl list-units --type=service --state=running 2>/dev/null | wc -l || service --status-all 2>/dev/null | grep running | wc -l || echo '0'",
		"servicesInstalled": "systemctl list-unit-files --type=service 2>/dev/null | wc -l || ls /etc/init.d/ 2>/dev/null | wc -l || echo '0'",
	}
}

// buildStatusScript joins all probe commands into a single remote command,
// each result introduced by a marker line.
func buildStatusScript(commands map[string]string, marker string) string {
	var probes []string
	for field, cmd := range commands {
		probes = append(probes, fmt.Sprintf("printf '\\n%s%s\\n'; { %s; } 2>/dev/null", marker, field, cmd))
	}
	return strings.Join(probes, "; ") + "; true"
}

// parseStatusScriptOutput parses the output of the batched status script,
// extracting field values introduced by marker lines.
func parseStatusScriptOutput(output, marker string) map[string]string {
	values := make(map[string]string)
	normalized := strings.ReplaceAll(output, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// Pattern: optional newline, marker, fieldname, newline
	pattern := regexp.MustCompile(`\n?` + regexp.QuoteMeta(marker) + `(\w+)\n`)
	matches := pattern.FindAllStringSubmatchIndex(normalized, -1)

	for i, match := range matches {
		fieldName := normalized[match[2]:match[3]]
		valueStart := match[1] // end of the full match (including the trailing \n)
		var valueEnd int
		if i+1 < len(matches) {
			valueEnd = matches[i+1][0] // start of next match
		} else {
			valueEnd = len(normalized)
		}
		values[fieldName] = strings.TrimSpace(normalized[valueStart:valueEnd])
	}

	return values
}
