import { useState, type FormEvent } from "react";
import { api } from "../api";
import { errorMessage, useAuth } from "../state";
import { Field } from "../components/ui";

export function Setup() {
  const { refresh } = useAuth();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [endpointHost, setEndpointHost] = useState(window.location.hostname === "localhost" ? "" : window.location.hostname);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (password !== confirm) {
      setError("The passwords do not match.");
      return;
    }
    setBusy(true);
    try {
      await api.setup({ username, password, endpointHost });
      await refresh();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <form className="card" onSubmit={submit}>
        <div className="card-body">
          <div className="brand">
            <div className="brand-mark" aria-hidden="true">
              <svg width="18" height="18" viewBox="0 0 32 32">
                <path d="M7 10l4 12 5-9 5 9 4-12" fill="none" stroke="currentColor" strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </div>
            <div className="brand-name">WGX</div>
          </div>
          <h1>Welcome</h1>
          <p className="muted" style={{ textAlign: "center", marginBottom: 16 }}>
            Create the first administrator. This form only works once.
          </p>
          {error && <div className="error">{error}</div>}
          <Field label="Username">
            <input className="input" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
          </Field>
          <Field label="Password" hint="At least 12 characters. Length beats complexity.">
            <input className="input" type="password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={12} />
          </Field>
          <Field label="Confirm password">
            <input className="input" type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required />
          </Field>
          <Field label="Public endpoint" hint="The hostname or IP address clients will connect to. You can change it later in Settings.">
            <input className="input" value={endpointHost} onChange={(e) => setEndpointHost(e.target.value)} placeholder="vpn.example.com" required />
          </Field>
          <button className="btn primary" type="submit" disabled={busy} style={{ width: "100%", justifyContent: "center" }}>
            {busy ? "…" : "Create administrator"}
          </button>
          <p className="small faint" style={{ textAlign: "center", margin: "14px 0 0" }}>
            <a href="https://github.com/Coffey-Labs/WGX" target="_blank" rel="noreferrer">
              AGPL-3.0 source
            </a>
          </p>
        </div>
      </form>
    </div>
  );
}
