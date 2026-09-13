package engine

import (
	"time"

	"github.com/Coffey-Labs/ihasvpn/internal/netcfg"
)

// SysctlStatus is one sysctl as reported to the UI.
type SysctlStatus struct {
	Key      string `json:"key"`
	Wanted   string `json:"wanted"`
	Current  string `json:"current"`
	Applied  bool   `json:"applied"`
	Required bool   `json:"required"`
	Why      string `json:"why"`
	Error    string `json:"error,omitempty"`
}

// Status is the server overview.
type Status struct {
	Version       string         `json:"version"`
	Backend       string         `json:"backend"`
	Interface     string         `json:"interface"`
	PublicKey     string         `json:"publicKey"`
	ListenPort    int            `json:"listenPort"`
	Addresses     []string       `json:"addresses"`
	Subnet4       string         `json:"subnet4"`
	Subnet6       string         `json:"subnet6,omitempty"`
	Egress        string         `json:"egress,omitempty"`
	FirewallError string         `json:"firewallError,omitempty"`
	FirewallManaged bool         `json:"firewallManaged"`
	StartedAt     time.Time      `json:"startedAt"`
	Sysctls       []SysctlStatus `json:"sysctls"`
	Totals        Totals         `json:"totals"`
	Settings      Settings       `json:"settings"`
}

// Version is stamped at build time.
var Version = "dev"

// Status assembles the overview.
func (e *Engine) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	st := Status{
		Version:         Version,
		Backend:         e.be.Kind(),
		Interface:       e.cfg.Iface,
		PublicKey:       e.serverKey.PublicKey().String(),
		ListenPort:      e.cfg.ListenPort,
		Subnet4:         e.cfg.Subnet4.Masked().String(),
		Egress:          e.egress,
		FirewallError:   e.fwErr,
		FirewallManaged: e.cfg.ManageFirewall && e.be.Kind() != "mock",
		StartedAt:       e.startedAt,
		Settings:        e.settings,
		Sysctls:         []SysctlStatus{},
	}
	if e.cfg.Subnet6.IsValid() {
		st.Subnet6 = e.cfg.Subnet6.Masked().String()
	}
	for _, a := range e.ServerAddresses() {
		st.Addresses = append(st.Addresses, a.String())
	}
	for _, r := range e.sysctls {
		st.Sysctls = append(st.Sysctls, sysctlStatus(r))
	}
	st.Totals = e.col.Snapshot().Totals
	return st
}

func sysctlStatus(r netcfg.Result) SysctlStatus {
	return SysctlStatus{Key: r.Key, Wanted: r.Value, Current: r.Current, Applied: r.Applied, Required: r.Required, Why: r.Why, Error: r.Err}
}
