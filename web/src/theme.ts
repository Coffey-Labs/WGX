// Theme preference: dark by default, light on request, or follow the OS.
// The choice is a per-browser convenience kept in localStorage; the resolved
// theme lands on <html data-theme> so the stylesheet needs only one selector.

export type ThemeChoice = "dark" | "light" | "system";

const KEY = "wgx.theme";
const media = window.matchMedia("(prefers-color-scheme: light)");

export function getThemeChoice(): ThemeChoice {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "system" || v === "dark") return v;
  } catch {
    /* storage unavailable */
  }
  return "dark";
}

function resolve(choice: ThemeChoice): "dark" | "light" {
  if (choice === "system") return media.matches ? "light" : "dark";
  return choice;
}

export function applyTheme(choice: ThemeChoice) {
  document.documentElement.dataset.theme = resolve(choice);
}

export function setThemeChoice(choice: ThemeChoice) {
  try {
    localStorage.setItem(KEY, choice);
  } catch {
    /* storage unavailable */
  }
  applyTheme(choice);
}

// Called once at startup: paints the right theme before React renders and
// keeps "system" honest when the OS switches.
export function initTheme() {
  applyTheme(getThemeChoice());
  media.addEventListener("change", () => {
    if (getThemeChoice() === "system") applyTheme("system");
  });
}
