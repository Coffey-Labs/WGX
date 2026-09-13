// Theme preference: dark by default, light on request, or follow the OS.
// The choice is a per-browser convenience kept in localStorage; the resolved
// theme lands on <html data-theme> so the stylesheet needs only one selector.

export type ThemeChoice = "dark" | "light" | "system";

const KEY = "wgx.theme";
const media = window.matchMedia("(prefers-color-scheme: light)");
const listeners = new Set<(c: ThemeChoice) => void>();

export function getThemeChoice(): ThemeChoice {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "system" || v === "dark") return v;
  } catch {
    /* storage unavailable */
  }
  return "dark";
}

export function resolveTheme(choice: ThemeChoice): "dark" | "light" {
  if (choice === "system") return media.matches ? "light" : "dark";
  return choice;
}

export function applyTheme(choice: ThemeChoice) {
  const resolved = resolveTheme(choice);
  document.documentElement.dataset.theme = resolved;
  // Keeps the browser chrome (address bar on phones) in step with the page.
  const meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.setAttribute("content", resolved === "light" ? "#f4f6f5" : "#121a17");
}

export function setThemeChoice(choice: ThemeChoice) {
  try {
    localStorage.setItem(KEY, choice);
  } catch {
    /* storage unavailable */
  }
  applyTheme(choice);
  listeners.forEach((l) => l(choice));
}

// Every switch on the page shares one choice; this keeps them in step.
export function subscribeTheme(l: (c: ThemeChoice) => void): () => void {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

// Called once at startup: paints the right theme before React renders and
// keeps "system" honest when the OS switches.
export function initTheme() {
  applyTheme(getThemeChoice());
  media.addEventListener("change", () => {
    if (getThemeChoice() === "system") applyTheme("system");
  });
}
