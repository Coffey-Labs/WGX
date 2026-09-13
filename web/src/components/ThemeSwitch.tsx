import { useEffect, useState } from "react";
import { Monitor, Moon, Sun } from "lucide-react";
import { getThemeChoice, setThemeChoice, subscribeTheme, type ThemeChoice } from "../theme";

const options: { value: ThemeChoice; label: string; icon: typeof Moon }[] = [
  { value: "dark", label: "Dark", icon: Moon },
  { value: "light", label: "Light", icon: Sun },
  { value: "system", label: "System", icon: Monitor },
];

function useThemeChoice(): [ThemeChoice, (v: ThemeChoice) => void] {
  const [choice, setChoice] = useState<ThemeChoice>(getThemeChoice);
  useEffect(() => subscribeTheme(setChoice), []);
  return [choice, setThemeChoice];
}

// Dark / Light / System as a segmented control. `compact` shows icons only;
// `icons={false}` shows labels only, for narrow places like the user menu.
export function ThemeSwitch({ compact = false, icons = true }: { compact?: boolean; icons?: boolean }) {
  const [choice, pick] = useThemeChoice();
  return (
    <div className="segmented" role="radiogroup" aria-label="Theme">
      {options.map((o) => {
        const Icon = o.icon;
        return (
          <button key={o.value} type="button" role="radio" aria-checked={choice === o.value} aria-label={o.label} title={o.label} className={choice === o.value ? "active" : ""} onClick={() => pick(o.value)}>
            {icons && <Icon size={14} style={{ verticalAlign: -2 }} />}
            {!compact && <span style={{ marginLeft: icons ? 6 : 0 }}>{o.label}</span>}
          </button>
        );
      })}
    </div>
  );
}

// One button that steps dark → light → system. Used in the shell where a
// three-way control would crowd the bar; the full switch lives in the user
// menu and on the Account page.
export function ThemeToggle() {
  const [choice, pick] = useThemeChoice();
  const idx = options.findIndex((o) => o.value === choice);
  const current = options[idx] ?? options[0];
  const next = options[(idx + 1) % options.length];
  const Icon = current.icon;
  return (
    <button type="button" className="icon-btn" title={`Theme: ${current.label}. Switch to ${next.label.toLowerCase()}`} aria-label={`Theme: ${current.label}. Switch to ${next.label.toLowerCase()}`} onClick={() => pick(next.value)}>
      <Icon size={17} />
    </button>
  );
}
