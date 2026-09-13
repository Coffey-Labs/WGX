import { useState, type FormEvent } from "react";
import { api } from "../api";
import { errorMessage, useAuth } from "../state";
import { Field } from "../components/ui";
import { Mark } from "../components/Mark";
import { Legal } from "../components/Legal";

export function Login() {
  const { refresh } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [stage, setStage] = useState<"password" | "totp">("password");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      if (stage === "password") {
        const r = await api.login(username, password);
        if (r.totpRequired) {
          setStage("totp");
          return;
        }
      } else {
        await api.loginTotp(code);
      }
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
            <div className="brand-mark">
              <Mark size={44} />
            </div>
            <div className="brand-name">ihasvpn</div>
          </div>
          <h1>{stage === "password" ? "Sign in" : "Second factor"}</h1>
          {error && <div className="error">{error}</div>}
          {stage === "password" ? (
            <>
              <Field label="Username">
                <input className="input" autoFocus autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
              </Field>
              <Field label="Password">
                <input className="input" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
              </Field>
            </>
          ) : (
            <Field label="Authenticator code" hint="Or one of your recovery codes.">
              <input className="input" autoFocus autoComplete="one-time-code" inputMode="numeric" value={code} onChange={(e) => setCode(e.target.value)} required />
            </Field>
          )}
          <button className="btn primary" type="submit" disabled={busy} style={{ width: "100%", justifyContent: "center" }}>
            {busy ? "…" : stage === "password" ? "Sign in" : "Verify"}
          </button>
          <Legal center />
        </div>
      </form>
    </div>
  );
}
