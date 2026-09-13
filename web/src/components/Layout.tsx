import type { ReactNode } from "react";
import { Link, useLocation } from "wouter";
import { Activity, ClipboardList, LayoutDashboard, LogOut, Settings, Shield, Users, Wifi, WifiOff } from "lucide-react";
import { useAuth, useLive } from "../state";

const items = [
  { href: "/", label: "Dashboard", icon: LayoutDashboard },
  { href: "/peers", label: "Peers", icon: Activity },
  { href: "/settings", label: "Settings", icon: Settings },
  { href: "/users", label: "Users", icon: Users },
  { href: "/audit", label: "Audit log", icon: ClipboardList },
  { href: "/account", label: "Account", icon: Shield },
];

export function Layout({ children }: { children: ReactNode }) {
  const [location] = useLocation();
  const { me, signOut } = useAuth();
  const { connected } = useLive();
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark" aria-hidden="true">
            <svg width="18" height="18" viewBox="0 0 32 32">
              <path d="M7 10l4 12 5-9 5 9 4-12" fill="none" stroke="currentColor" strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <div>
            <div className="brand-name">WGX</div>
            <div className="brand-sub">WireGuard server</div>
          </div>
        </div>
        <nav className="nav">
          {items.map((it) => {
            const active = it.href === "/" ? location === "/" : location.startsWith(it.href);
            const Icon = it.icon;
            return (
              <Link key={it.href} href={it.href} className={active ? "active" : ""}>
                <Icon />
                <span>{it.label}</span>
              </Link>
            );
          })}
        </nav>
        <div className="sidebar-foot">
          <div title={connected ? "Live updates connected" : "Live updates reconnecting"} className="nowrap">
            {connected ? <Wifi size={13} style={{ verticalAlign: -2, color: "var(--ok)" }} /> : <WifiOff size={13} style={{ verticalAlign: -2, color: "var(--warn)" }} />} {connected ? "live" : "reconnecting"}
          </div>
          <div className="nowrap">
            {me?.username} <span className="faint">({me?.role})</span>
          </div>
          <button className="btn sm ghost" onClick={() => void signOut()} style={{ alignSelf: "flex-start", marginLeft: -6 }}>
            <LogOut /> Sign out
          </button>
          <a className="nowrap" href="https://github.com/Coffey-Labs/WGX" target="_blank" rel="noreferrer">
            AGPL-3.0 source
          </a>
        </div>
      </aside>
      <main className="main">{children}</main>
    </div>
  );
}
