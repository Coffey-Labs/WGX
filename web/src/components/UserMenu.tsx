import { useEffect, useRef, useState } from "react";
import { Link } from "wouter";
import { ChevronDown, LogOut, Shield } from "lucide-react";
import { useAuth } from "../state";
import { ThemeSwitch } from "./ThemeSwitch";

// The signed-in user: avatar button that opens a small menu with the
// account page, the theme switch and sign out. `placement` says which way
// the menu opens; the sidebar footer opens upward, the top bar downward.
export function UserMenu({ placement, showName = false }: { placement: "up" | "down"; showName?: boolean }) {
  const { me, signOut } = useAuth();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  if (!me) return null;
  const initial = me.username.slice(0, 1).toUpperCase();
  return (
    <div className={`user-menu ${placement}`} ref={root}>
      <button type="button" className={`user-btn${showName ? " wide" : ""}`} aria-haspopup="menu" aria-expanded={open} aria-label={showName ? undefined : `Account menu for ${me.username}`} onClick={() => setOpen((v) => !v)}>
        <span className="avatar" aria-hidden="true">
          {initial}
        </span>
        {showName && (
          <>
            <span className="user-text">
              <span className="user-name">{me.username}</span>
              <span className="user-role">{me.role}</span>
            </span>
            <ChevronDown size={15} className="user-chev" />
          </>
        )}
      </button>
      {open && (
        <div className="menu" role="menu">
          <div className="menu-head">
            <span className="avatar lg" aria-hidden="true">
              {initial}
            </span>
            <div className="user-text">
              <span className="user-name">{me.username}</span>
              <span className="user-role">{me.role}</span>
            </div>
          </div>
          <Link href="/account" role="menuitem" className="menu-item" onClick={() => setOpen(false)}>
            <Shield size={16} /> Account &amp; security
          </Link>
          <div className="menu-row">
            <span className="menu-label">Theme</span>
            <ThemeSwitch icons={false} />
          </div>
          <button type="button" role="menuitem" className="menu-item" onClick={() => void signOut()}>
            <LogOut size={16} /> Sign out
          </button>
        </div>
      )}
    </div>
  );
}
