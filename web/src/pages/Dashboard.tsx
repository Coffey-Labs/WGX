import { useEffect, useState } from "react";
import { Link } from "wouter";
import { AlertTriangle, Copy } from "lucide-react";
import { api, type Peer, type Range, type Status, type TrafficPoint } from "../api";
import { ago, bytes, duration, rate } from "../format";
import { errorMessage, useLive, useNow, useToast } from "../state";
import { Legend, TrafficChart } from "../components/charts";
import { Segmented, copyText } from "../components/ui";

const rangeMs: Record<Range, number> = { "1h": 3600e3, "24h": 86400e3, "7d": 7 * 86400e3, "30d": 30 * 86400e3 };

export function Dashboard() {
  const { snapshot, peersVersion } = useLive();
  const toast = useToast();
  const now = useNow();
  const [status, setStatus] = useState<Status | null>(null);
  const [peers, setPeers] = useState<Peer[]>([]);
  const [range, setRange] = useState<Range>("24h");
  const [series, setSeries] = useState<TrafficPoint[]>([]);
  const [showSysctls, setShowSysctls] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.status().then(setStatus).catch((e) => setError(errorMessage(e)));
  }, []);
  useEffect(() => {
    api.peers().then(setPeers).catch((e) => setError(errorMessage(e)));
  }, [peersVersion]);
  useEffect(() => {
    let live = true;
    const load = () => api.usage(range).then((s) => live && setSeries(s)).catch(() => {});
    void load();
    const t = setInterval(load, 60_000);
    return () => {
      live = false;
      clearInterval(t);
    };
  }, [range]);

  const totals = snapshot?.totals ?? status?.totals;
  const connected = peers
    .map((p) => ({ p, l: snapshot?.peers[p.id] ?? p.live }))
    .filter((x) => x.l.connected)
    .sort((a, b) => b.l.rxRate + b.l.txRate - (a.l.rxRate + a.l.txRate));
  const unapplied = status?.sysctls.filter((s) => !s.applied) ?? [];

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Dashboard</h1>
          <p>{status ? `${status.interface} on UDP ${status.listenPort} · ${status.backend} data plane` : " "}</p>
        </div>
      </div>
      {error && <div className="error">{error}</div>}
      {status?.backend === "userspace" && (
        <div className="notice">
          <AlertTriangle size={14} style={{ verticalAlign: -2 }} /> Running on the userspace data plane (wireguard-go). Load the <code>wireguard</code> kernel module on the host for several times the throughput.
        </div>
      )}
      {status?.backend === "mock" && <div className="notice">Mock data plane: no real tunnel exists. Traffic and handshakes are simulated.</div>}
      {status?.firewallError && (
        <div className="error">
          <AlertTriangle size={14} style={{ verticalAlign: -2 }} /> Firewall rules were not applied: {status.firewallError}. Peers will connect but cannot reach beyond the server.
        </div>
      )}
      {unapplied.some((s) => s.required) && (
        <div className="error">
          <AlertTriangle size={14} style={{ verticalAlign: -2 }} /> IP forwarding is off and could not be enabled. Pass <code>net.ipv4.ip_forward=1</code> in the container's sysctls.
        </div>
      )}

      <div className="grid grid-4">
        <Stat label="Connected" value={`${totals?.connected ?? 0}`} sub={`of ${totals?.active ?? 0} enabled · ${totals?.peers ?? 0} total`} />
        <Stat label="Throughput" value={rate((totals?.rxRate ?? 0) + (totals?.txRate ?? 0))} sub={`↓ ${rate(totals?.rxRate ?? 0)} · ↑ ${rate(totals?.txRate ?? 0)}`} />
        <Stat label="Received" value={bytes(totals?.rx ?? 0)} sub="from peers, all time" />
        <Stat label="Sent" value={bytes(totals?.tx ?? 0)} sub="to peers, all time" />
      </div>

      <div className="grid grid-2 mt" style={{ gridTemplateColumns: "2fr 1fr" }}>
        <div className="card">
          <div className="card-head">
            <h2>Traffic</h2>
            <div className="toolbar">
              <Legend />
              <Segmented value={range} onChange={setRange} options={[{ value: "1h", label: "1h" }, { value: "24h", label: "24h" }, { value: "7d", label: "7d" }, { value: "30d", label: "30d" }]} />
            </div>
          </div>
          <div className="card-body">
            <TrafficChart points={series} from={now - rangeMs[range]} to={now} bucketSeconds={range === "30d" ? 3600 : 300} />
            <div className="small faint">Five-minute buckets{range === "30d" ? ", shown per hour" : ""}. Live rates above update every couple of seconds.</div>
          </div>
        </div>
        <div className="card">
          <div className="card-head">
            <h2>Server</h2>
          </div>
          <div className="card-body">
            {status && (
              <dl className="kv">
                <dt>Public key</dt>
                <dd className="mono">
                  {status.publicKey}{" "}
                  <button className="btn icon ghost sm" title="Copy" onClick={() => copyText(status.publicKey).then((ok) => toast(ok ? "Copied" : "Could not copy", ok ? "ok" : "bad"))}>
                    <Copy />
                  </button>
                </dd>
                <dt>Endpoint</dt>
                <dd className="mono">
                  {status.settings.endpointHost}:{status.settings.endpointPort}
                </dd>
                <dt>Tunnel</dt>
                <dd className="mono">{status.addresses.join(", ")}</dd>
                <dt>MTU</dt>
                <dd>{status.settings.mtu}</dd>
                <dt>Egress</dt>
                <dd>{status.egress || (status.firewallManaged ? "any" : "not managed")}</dd>
                <dt>Firewall</dt>
                <dd>{status.firewallManaged ? (status.firewallError ? <span className="badge bad">failed</span> : <span className="badge ok">nftables</span>) : <span className="badge">host-managed</span>}</dd>
                <dt>Up since</dt>
                <dd>{duration(status.startedAt, now) || "—"}</dd>
                <dt>Version</dt>
                <dd>{status.version}</dd>
              </dl>
            )}
            {status && status.sysctls.length > 0 && (
              <div className="mt small">
                <button className="btn sm ghost" onClick={() => setShowSysctls((v) => !v)} style={{ marginLeft: -8 }}>
                  {showSysctls ? "Hide" : "Show"} kernel tuning ({status.sysctls.length - unapplied.length}/{status.sysctls.length} applied)
                </button>
                {showSysctls && (
                  <div className="table-wrap mt">
                    <table>
                      <thead>
                        <tr>
                          <th>sysctl</th>
                          <th>wanted</th>
                          <th>current</th>
                        </tr>
                      </thead>
                      <tbody>
                        {status.sysctls.map((s) => (
                          <tr key={s.key} title={s.error ? `${s.why}. ${s.error}` : s.why}>
                            <td className="mono">{s.key}</td>
                            <td className="mono">{s.wanted}</td>
                            <td className="mono">
                              {s.current || "?"} {s.applied ? <span className="badge ok">ok</span> : <span className={`badge ${s.required ? "bad" : "warn"}`}>not set</span>}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                    {unapplied.length > 0 && <p className="faint mt">Values marked "not set" are global sysctls the container may not change. Apply them on the host; see docs/performance.md.</p>}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="card mt">
        <div className="card-head">
          <h2>Connected now</h2>
          <Link href="/peers" className="small">
            All peers →
          </Link>
        </div>
        {connected.length === 0 ? (
          <div className="empty">No peer has handshaken in the last {status?.settings.connectedWindow ?? 180} seconds.</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Peer</th>
                  <th>Address</th>
                  <th>Endpoint</th>
                  <th>Session</th>
                  <th>Handshake</th>
                  <th className="right">Rate</th>
                  <th className="right">Transfer</th>
                </tr>
              </thead>
              <tbody>
                {connected.map(({ p, l }) => (
                  <tr key={p.id} className="clickable" onClick={() => (window.location.hash = "")}>
                    <td>
                      <Link href={`/peers/${p.id}`}>
                        <span className="dot on" />
                        {p.name}
                      </Link>
                    </td>
                    <td className="mono">{p.ipv4}</td>
                    <td className="mono">{l.endpoint ?? "—"}</td>
                    <td>{duration(l.connectedSince, now)}</td>
                    <td>{ago(l.lastHandshake, now)}</td>
                    <td className="num right">
                      ↓ {rate(l.rxRate)} · ↑ {rate(l.txRate)}
                    </td>
                    <td className="num right">
                      {bytes(l.rx)} / {bytes(l.tx)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );
}

function Stat({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="card stat">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {sub && <div className="stat-sub">{sub}</div>}
    </div>
  );
}
