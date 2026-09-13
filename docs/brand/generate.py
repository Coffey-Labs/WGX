#!/usr/bin/env python3
"""Generate the ihasvpn brand set from one drawing.

Writes docs/brand/* and the favicons and app icons under web/public. The same
drawing is inlined in web/src/components/Mark.tsx; change both together.

Needs rsvg-convert, ImageMagick 7 (`magick`) and the Fira Sans font, the face
the ihasmail.org cards use. Run from anywhere:

    python3 docs/brand/generate.py
"""
import atexit, os, shutil, subprocess, tempfile

REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
OUT = os.path.join(REPO, "docs/brand")
PUB = os.path.join(REPO, "web/public")
TMP = tempfile.mkdtemp(prefix="ihasvpn-brand-")
atexit.register(shutil.rmtree, TMP, ignore_errors=True)

NAVY, TEAL, ORANGE, EAR = "#17404f", "#46cac3", "#f9a34b", "#ef7f2f"
SHIELD = "M64 6 L114 22 V60 C114 90 93 112 64 122 C35 112 14 90 14 60 V22 Z"
HEAD = ("M31 64 C30 50 31 38 34 27 Q36 20 42 23 L55 32 Q64 29.5 73 32 L86 23 "
        "Q92 20 94 27 C97 38 98 50 97 64 C96 83 82 92 64 92 C46 92 32 83 31 64 Z")
HEAD_T = "translate(64 62) scale(0.9) translate(-64 -60)"
EARS = "M38 30 L50 37.5 L40 45.5 Z M90 30 L78 37.5 L88 45.5 Z"
EYES = "M44 60 Q49.5 53 55 60 M73 60 Q78.5 53 84 60"
NOSE = "M60.8 65.5 h6.4 l-3.2 3.6 z"
MOUTH = "M56 71.5 Q60 76.5 64 71.5 Q68 76.5 72 71.5"
WHISKERS = "M14 58 L30 60 M15 67 L30 65 M114 58 L98 60 M113 67 L98 65"
LEDGE = "M6 88 Q64 81 122 88"
PAW_L = "M36.5 90 C36 81 40 76.5 46 76.5 C52 76.5 56 81 55.5 90 C55.5 94 51 96 46 96 C41 96 36.5 94 36.5 90 Z"
PAW_R = "M91.5 90 C92 81 88 76.5 82 76.5 C76 76.5 72 81 72.5 90 C72.5 94 77 96 82 96 C87 96 91.5 94 91.5 90 Z"
TOES = "M43 88.5 V93 M49 88.5 V93 M79 88.5 V93 M85 88.5 V93"

def mark_body(pfx):
    """The colour mark's drawing, ids prefixed so several can share a page."""
    return f'''<defs><clipPath id="{pfx}-clip"><path d="{SHIELD}"/></clipPath></defs>
  <path d="{SHIELD}" fill="{TEAL}"/>
  <g clip-path="url(#{pfx}-clip)" stroke="{NAVY}" stroke-linecap="round" stroke-linejoin="round">
    <path d="{WHISKERS}" stroke-width="3" fill="none"/>
    <g transform="{HEAD_T}">
      <path d="{HEAD}" fill="{ORANGE}" stroke-width="5"/>
      <path d="{EARS}" fill="{EAR}" stroke="none"/>
      <path d="{EYES}" stroke-width="4.2" fill="none"/>
      <path d="{NOSE}" fill="{NAVY}" stroke-width="2"/>
      <path d="{MOUTH}" stroke-width="3.5" fill="none"/>
    </g>
    <path d="{LEDGE} L122 130 L6 130 Z" fill="{TEAL}" stroke-width="4.5"/>
    <path d="{PAW_L}" fill="{ORANGE}" stroke-width="4"/>
    <path d="{PAW_R}" fill="{ORANGE}" stroke-width="4"/>
    <path d="{TOES}" stroke-width="2.4" fill="none"/>
  </g>
  <path d="{SHIELD}" fill="none" stroke="{NAVY}" stroke-width="6" stroke-linejoin="round"/>'''

def svg(body, w=128, h=128, vb="0 0 128 128", label="ihasvpn", comment=""):
    c = f"\n  <!-- {comment} -->" if comment else ""
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{vb}" width="{w}" height="{h}" role="img" aria-label="{label}">{c}\n  {body}\n</svg>\n'

MARK = svg(mark_body("ihasvpn"), comment="ihasvpn: the ihasmail cat peeking over the edge of a shield")

MONO = svg(f'''<defs>
    <clipPath id="m-clip"><path d="{SHIELD}"/></clipPath>
    <clipPath id="m-above"><path d="M0 0 H128 V88 Q64 81 0 88 Z"/></clipPath>
    <mask id="m-paws"><rect width="128" height="128" fill="#fff"/><path d="{PAW_L} {PAW_R}" fill="#000" stroke="#000" stroke-width="4"/></mask>
  </defs>
  <g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round">
    <path d="{SHIELD}" stroke-width="7"/>
    <g clip-path="url(#m-clip)">
      <g clip-path="url(#m-above)">
        <path d="{WHISKERS}" stroke-width="3"/>
        <g transform="{HEAD_T}">
          <path d="{HEAD}" stroke-width="5"/>
          <path d="{EYES}" stroke-width="4.2"/>
          <path d="{MOUTH}" stroke-width="3.5"/>
        </g>
      </g>
      <path d="{LEDGE}" stroke-width="4.5" mask="url(#m-paws)"/>
      <path d="{PAW_L}" stroke-width="4"/>
      <path d="{PAW_R}" stroke-width="4"/>
      <path d="{TOES}" stroke-width="2.4"/>
    </g>
  </g>
  <path transform="{HEAD_T}" d="{NOSE}" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>''',
    comment="single-colour version: inherits currentColor, for print, status bars and anywhere one ink")

def write(path, text):
    with open(path, "w") as f:
        f.write(text)

def render(src, dst, w, h=None):
    subprocess.run(["rsvg-convert", "-w", str(w), "-h", str(h or w), src, "-o", dst], check=True)

def magick(*args):
    subprocess.run(["magick", *args], check=True)

write(f"{OUT}/ihasvpn-mark.svg", MARK)
write(f"{OUT}/ihasvpn-mark-mono.svg", MONO)
write(f"{PUB}/favicon.svg", MARK)
render(f"{OUT}/ihasvpn-mark.svg", f"{OUT}/ihasvpn-mark-256.png", 256)
render(f"{OUT}/ihasvpn-mark.svg", f"{OUT}/ihasvpn-mark-1024.png", 1024)
# Mono PNG in the brand navy, since currentColor has no value outside a page.
write(f"{TMP}/mono-navy.svg", MONO.replace("currentColor", NAVY))
render(f"{TMP}/mono-navy.svg", f"{OUT}/ihasvpn-mark-mono-256.png", 256)

# Wordmarks: light ground (navy text) and dark ground (pale text).
for name, fg in (("ihasvpn-wordmark", "#10303d"), ("ihasvpn-wordmark-dark", "#eaf6f6")):
    body = f'''<g transform="translate(24 23) scale(2.36)">{mark_body("wm")}</g>
  <text x="340" y="228" font-family="Fira Sans" font-weight="800" font-size="168" letter-spacing="-5" fill="{fg}">ihasvpn</text>'''
    write(f"{TMP}/{name}.svg", svg(body, 936, 346, "0 0 936 346"))
    render(f"{TMP}/{name}.svg", f"{OUT}/{name}.png", 936, 346)

# Social card, GitHub's 1280x640, in the ihasmail.org card's layout.
social = f'''<defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="#13394a"/><stop offset="1" stop-color="#0a1c26"/>
    </linearGradient>
  </defs>
  <rect width="1280" height="640" fill="url(#bg)"/>
  <g transform="translate(120 88) scale(3.05)">{mark_body("sc")}</g>
  <text x="520" y="262" font-family="Fira Sans" font-weight="700" font-size="112" letter-spacing="-2" fill="#eaf6f6">ihasvpn</text>
  <text x="524" y="334" font-family="Fira Sans" font-size="38" fill="#46cac3">Self-hosted WireGuard server</text>
  <text x="524" y="382" font-family="Fira Sans" font-size="38" fill="#46cac3">with a secure web console</text>
  <text x="524" y="446" font-family="Fira Sans" font-size="31" fill="#a3c3cb">One container · kernel data plane · 2FA</text>
  <rect x="100" y="512" width="1080" height="2" fill="#21505f"/>
  <text x="100" y="572" font-family="Fira Sans" font-size="29" fill="#eaf6f6">github.com/Coffey-Labs/ihasvpn</text>
  <text x="1180" y="572" text-anchor="end" font-family="Fira Sans" font-size="29" fill="#f9a34b">AGPL-3.0 · Coffey Labs</text>'''
write(f"{TMP}/social.svg", svg(social, 1280, 640, "0 0 1280 640"))
render(f"{TMP}/social.svg", f"{OUT}/ihasvpn-social.png", 1280, 640)

# Favicons and app icons.
for s in (16, 32, 48, 192, 512):
    render(f"{OUT}/ihasvpn-mark.svg", f"{TMP}/fav-{s}.png", s)
magick(f"{TMP}/fav-16.png", f"{TMP}/fav-32.png", f"{TMP}/fav-48.png", f"{PUB}/favicon.ico")
magick(f"{TMP}/fav-32.png", f"{PUB}/favicon-32.png")
magick(f"{TMP}/fav-192.png", f"{PUB}/icon-192.png")
magick(f"{TMP}/fav-512.png", f"{PUB}/icon-512.png")
# iOS paints transparency black, so the touch icon gets the navy ground.
render(f"{OUT}/ihasvpn-mark.svg", f"{TMP}/fav-144.png", 144)
magick("-size", "180x180", "xc:#0d2430", f"{TMP}/fav-144.png", "-gravity", "center", "-composite", f"{PUB}/apple-touch-icon.png")
print("brand set written")
