import { useState } from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { getThemeChoice, setThemeChoice, type ThemeChoice } from "../theme";

const options: { value: ThemeChoice; label: string; icon: typeof Moon }[] = [
  { value: "dark", label: "Dark", icon: Moon },
  { value: "light", label: "Light", icon: Sun },
  { value: "system", label: "System", icon: Monitor },
];

// Dark / Light / System. `compact` shows icons only, for the sidebar.
export function ThemeSwitch({ compact = false }: { compact?: boolean }) {
  const [choice, setChoice] = useState<ThemeChoice>(getThemeChoice);
  const pick = (v: ThemeChoice) => {
    setThemeChoice(v);
    setChoice(v);
  };
  return (
    <div className="segmented" role="radiogroup" aria-label="Theme">
      {options.map((o) => {
        const Icon = o.icon;
        return (
          <button key={o.value} role="radio" aria-checked={choice === o.value} aria-label={o.label} title={o.label} className={choice === o.value ? "active" : ""} onClick={() => pick(o.value)}>
            <Icon size={14} style={{ verticalAlign: -2 }} />
            {!compact && <span style={{ marginLeft: 6 }}>{o.label}</span>}
          </button>
        );
      })}
    </div>
  );
}
