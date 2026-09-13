import { useId } from "react";

// The WGX mark: a padlock on a shield, with the W as its keyhole. The same
// drawing as docs/brand/wgx-mark.svg, inlined so it needs no request and
// takes its size from wherever it is placed. Gradient ids are per instance:
// the mark appears more than once on a page (sidebar, top bar), and a shared
// id would resolve to whichever copy comes first, which may be hidden and
// then paints nothing.
export function Mark({ size = 32 }: { size?: number }) {
  const id = useId();
  const shield = `${id}-shield`;
  const lock = `${id}-lock`;
  return (
    <svg width={size} height={size} viewBox="0 0 128 128" role="img" aria-label="WGX">
      <defs>
        <linearGradient id={shield} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#26332e" />
          <stop offset="1" stopColor="#121a17" />
        </linearGradient>
        <linearGradient id={lock} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#3fae90" />
          <stop offset="1" stopColor="#2b8f75" />
        </linearGradient>
      </defs>
      <path d="M64 6 L114 22 V60 C114 90 93 112 64 122 C35 112 14 90 14 60 V22 Z" fill={`url(#${shield})`} />
      <path d="M64 6 L114 22 V60 C114 90 93 112 64 122 C35 112 14 90 14 60 V22 Z" fill="none" stroke="#3fae90" strokeWidth="3" strokeLinejoin="round" />
      <path d="M46 62 V50 a18 18 0 0 1 36 0 V62" fill="none" stroke="#3fae90" strokeWidth="8.5" strokeLinecap="round" />
      <rect x="34" y="58" width="60" height="42" rx="10" fill={`url(#${lock})`} />
      <path d="M46 70 L53 89 L64 76 L75 89 L82 70" fill="none" stroke="#ffffff" strokeWidth="6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
