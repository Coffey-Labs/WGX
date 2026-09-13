package netcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sysctl is one kernel parameter and the value WGX wants for it.
type Sysctl struct {
	Key   string
	Value string
	// Required marks the ones the VPN cannot work without (forwarding). The
	// others are throughput tuning: nice to have, and often refused inside a
	// container because they are not network-namespaced.
	Required bool
	// Why is shown in the log and the UI when a value could not be set.
	Why string
}

// Result records what happened to one sysctl.
type Result struct {
	Sysctl
	Applied bool
	Current string
	Err     string
}

// Wanted returns the sysctls WGX applies at startup, in order.
func Wanted(ipv6 bool) []Sysctl {
	s := []Sysctl{
		{Key: "net.ipv4.ip_forward", Value: "1", Required: true, Why: "peers cannot reach anything beyond the server without forwarding"},
		// Strict reverse-path filtering drops replies that arrive on the
		// tunnel for a source the kernel would route elsewhere. Loose is
		// what every VPN gateway runs.
		{Key: "net.ipv4.conf.all.rp_filter", Value: "2", Why: "strict rp_filter drops legitimate tunnel replies"},
		{Key: "net.ipv4.conf.default.rp_filter", Value: "2", Why: "strict rp_filter drops legitimate tunnel replies"},
		// The remaining ones are throughput. They are global (not
		// namespaced), so inside a container they usually fail and must be
		// set on the host instead -- see docs/performance.md.
		{Key: "net.core.rmem_max", Value: "26214400", Why: "larger UDP receive buffers stop bursts being dropped before WireGuard reads them"},
		{Key: "net.core.wmem_max", Value: "26214400", Why: "larger UDP send buffers keep the encrypt path from stalling"},
		{Key: "net.core.rmem_default", Value: "1048576", Why: "default socket receive buffer"},
		{Key: "net.core.wmem_default", Value: "1048576", Why: "default socket send buffer"},
		{Key: "net.core.netdev_max_backlog", Value: "16384", Why: "deeper per-CPU input queue for 10GbE bursts"},
		{Key: "net.ipv4.udp_rmem_min", Value: "16384", Why: "minimum UDP receive buffer under memory pressure"},
		{Key: "net.ipv4.udp_wmem_min", Value: "16384", Why: "minimum UDP send buffer under memory pressure"},
	}
	if ipv6 {
		s = append(s, Sysctl{Key: "net.ipv6.conf.all.forwarding", Value: "1", Required: true, Why: "IPv6 peers cannot reach anything beyond the server without forwarding"})
	}
	return s
}

// ApplyAll writes each sysctl through /proc/sys and reports what happened.
// A value that is already right counts as applied. A required value that
// cannot be set is returned as an error along with the full report so the
// caller can decide whether to keep going.
func ApplyAll(want []Sysctl) ([]Result, error) {
	var results []Result
	var fatal []string
	for _, s := range want {
		r := Result{Sysctl: s}
		path := filepath.Join("/proc/sys", strings.ReplaceAll(s.Key, ".", "/"))
		cur, err := os.ReadFile(path)
		if err == nil {
			r.Current = strings.TrimSpace(string(cur))
		}
		if r.Current == s.Value {
			r.Applied = true
			results = append(results, r)
			continue
		}
		if err := os.WriteFile(path, []byte(s.Value), 0o644); err != nil {
			r.Err = err.Error()
			if s.Required && !forwardingSatisfied(r.Current, s.Value) {
				fatal = append(fatal, fmt.Sprintf("%s=%s (%s)", s.Key, s.Value, r.Err))
			}
		} else {
			r.Applied = true
			r.Current = s.Value
		}
		results = append(results, r)
	}
	if len(fatal) > 0 {
		return results, fmt.Errorf("required sysctls could not be set: %s -- pass them with `--sysctl` or the compose `sysctls:` list", strings.Join(fatal, ", "))
	}
	return results, nil
}

// forwardingSatisfied treats "1" as satisfied for forwarding keys even when
// the file was read-only, which is what a compose `sysctls:` entry produces.
func forwardingSatisfied(current, want string) bool { return current == want }
