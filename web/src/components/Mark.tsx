import { useId } from "react";

// The ihasvpn mark: the ihasmail cat peeking over the edge of a shield. The
// same drawing as docs/brand/ihasvpn-mark.svg (docs/brand/generate.py writes
// that one), inlined so it needs no request and takes its size from wherever
// it is placed. The clip-path id is per instance: the mark appears more than
// once on a page (sidebar, top bar), and a shared id resolves to whichever
// copy comes first, which may be hidden and then clips everything away.
const NAVY = "#17404f";
const TEAL = "#46cac3";
const ORANGE = "#f9a34b";
const SHIELD = "M64 6 L114 22 V60 C114 90 93 112 64 122 C35 112 14 90 14 60 V22 Z";

export function Mark({ size = 32 }: { size?: number }) {
  const clip = `${useId()}-clip`;
  return (
    <svg width={size} height={size} viewBox="0 0 128 128" role="img" aria-label="ihasvpn">
      <defs>
        <clipPath id={clip}>
          <path d={SHIELD} />
        </clipPath>
      </defs>
      <path d={SHIELD} fill={TEAL} />
      <g clipPath={`url(#${clip})`} stroke={NAVY} strokeLinecap="round" strokeLinejoin="round">
        <path d="M14 58 L30 60 M15 67 L30 65 M114 58 L98 60 M113 67 L98 65" strokeWidth="3" fill="none" />
        <g transform="translate(64 62) scale(0.9) translate(-64 -60)">
          <path d="M31 64 C30 50 31 38 34 27 Q36 20 42 23 L55 32 Q64 29.5 73 32 L86 23 Q92 20 94 27 C97 38 98 50 97 64 C96 83 82 92 64 92 C46 92 32 83 31 64 Z" fill={ORANGE} strokeWidth="5" />
          <path d="M38 30 L50 37.5 L40 45.5 Z M90 30 L78 37.5 L88 45.5 Z" fill="#ef7f2f" stroke="none" />
          <path d="M44 60 Q49.5 53 55 60 M73 60 Q78.5 53 84 60" strokeWidth="4.2" fill="none" />
          <path d="M60.8 65.5 h6.4 l-3.2 3.6 z" fill={NAVY} strokeWidth="2" />
          <path d="M56 71.5 Q60 76.5 64 71.5 Q68 76.5 72 71.5" strokeWidth="3.5" fill="none" />
        </g>
        <path d="M6 88 Q64 81 122 88 L122 130 L6 130 Z" fill={TEAL} strokeWidth="4.5" />
        <path d="M36.5 90 C36 81 40 76.5 46 76.5 C52 76.5 56 81 55.5 90 C55.5 94 51 96 46 96 C41 96 36.5 94 36.5 90 Z" fill={ORANGE} strokeWidth="4" />
        <path d="M91.5 90 C92 81 88 76.5 82 76.5 C76 76.5 72 81 72.5 90 C72.5 94 77 96 82 96 C87 96 91.5 94 91.5 90 Z" fill={ORANGE} strokeWidth="4" />
        <path d="M43 88.5 V93 M49 88.5 V93 M79 88.5 V93 M85 88.5 V93" strokeWidth="2.4" fill="none" />
      </g>
      <path d={SHIELD} fill="none" stroke={NAVY} strokeWidth="6" strokeLinejoin="round" />
    </svg>
  );
}
