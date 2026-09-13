import { useEffect, useState } from "react";
import { api, type AuditEntry } from "../api";
import { dateTime } from "../format";
import { errorMessage, useToast } from "../state";

export function Audit() {
  const toast = useToast();
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [query, setQuery] = useState("");

  useEffect(() => {
    api.audit(500).then(setEntries).catch((e) => toast(errorMessage(e), "bad"));
  }, [toast]);

  const q = query.trim().toLowerCase();
  const rows = q ? entries.filter((e) => [e.actor, e.action, e.target, e.detail, e.ip].some((v) => v.toLowerCase().includes(q))) : entries;

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Audit log</h1>
          <p>Every administrative action, newest first. The last 5,000 entries are kept.</p>
        </div>
        <input className="input search" placeholder="Filter…" value={query} onChange={(e) => setQuery(e.target.value)} />
      </div>
      <div className="card">
        {rows.length === 0 ? (
          <div className="empty">Nothing recorded yet.</div>
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>When</th>
                  <th>Who</th>
                  <th>Action</th>
                  <th>Target</th>
                  <th>Detail</th>
                  <th>From</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((e) => (
                  <tr key={e.id}>
                    <td className="nowrap">{dateTime(e.at)}</td>
                    <td>{e.actor}</td>
                    <td>
                      <span className={`badge ${e.action.includes("failed") ? "bad" : e.action.includes("deleted") || e.action.includes("disabled") ? "warn" : ""}`}>{e.action}</span>
                    </td>
                    <td>{e.target}</td>
                    <td className="muted">{e.detail}</td>
                    <td className="mono">{e.ip}</td>
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
