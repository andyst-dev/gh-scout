#!/usr/bin/env python3
from PIL import Image, ImageDraw, ImageFont

FONT = "/System/Library/Fonts/SFNSMono.ttf"
BG=(19,23,26); TITLE=(35,40,48); FG=(230,237,243)
GREEN=(63,185,80); BLUE=(88,166,255); RED=(248,81,73); GRAY=(139,148,158)

def textw(d, t, font): return d.textlength(t, font=font)

def render(name, layout, fs=18, pad=26, tab_h=0, title="gh-scout"):
    font = ImageFont.truetype(FONT, fs)
    size = fs*1.45
    d0 = ImageDraw.Draw(Image.new("RGB",(8,8)))
    maxw = 0
    for line in layout:
        w = sum(textw(d0, seg[1], font) for seg in line)
        maxw = max(maxw, w)
    lh = int(size); W=int(maxw)+pad*2; H=int(pad*2 + lh*len(layout)) + (tab_h or 0)
    img = Image.new("RGB",(W,H), BG); d=ImageDraw.Draw(img)
    if tab_h:
        d.rectangle([0,0,W,tab_h], fill=TITLE)
        r=max(4,int(fs*0.28))
        for cx,col in ((int(fs*0.5),(255,95,86)),(int(fs*1.2),(255,189,46)),(int(fs*1.9),(39,201,63))):
            d.ellipse([cx, tab_h/2-r, cx+2*r, tab_h/2+r], fill=col)
        tw=textw(d,title,font); d.text(((W-tw)/2,(tab_h-fs*0.85)/2), title, font=font, fill=GRAY)
    y=pad+(tab_h or 0)
    for line in layout:
        x=pad
        for col,t in line:
            d.text((x,y), t, font=font, fill=col); x+=textw(d,t,font)
        y+=lh
    img.save(name); print(name, f"{W}x{H}")

ART=["          __                             __",
     "   ____ _/ /_     ______________  __  __/ /_",
     "  / __ `/ __ \\   / ___/ ___/ __ \\/ / / / __/",
     " / /_/ / / / /  (__  ) /__/ /_/ / /_/ / /_",
     " \\__, /_/ /_/  /____/\\___/\\____/\\__,_/\\__/",
     "/____/"]

def line(col,t): return [(col,t)]

# header banner
render("assets/banner.png", [line(GREEN,a) for a in ART], fs=26, pad=40)

# full session
session=[
 [ (GREEN,"❯ "),(GREEN,"gh-scout --since 30 acme/widgets acme/gadgets") ],
 [ (BG,"") ],
 line(GREEN,ART[0]),line(GREEN,ART[1]),line(GREEN,ART[2]),
 line(GREEN,ART[3]),line(GREEN,ART[4]),line(GREEN,ART[5]),
 line(GREEN,"  gh-scout · contribution issues without duplicate work"),
 line(GREEN,"  score = fixability 0-100 · defects, reproductions, tests & help labels help"),
 [ (BG,"") ],
 line(BLUE,"## Ready targets · 4"),
 [ (GREEN,"[#482 · score 78]"),(FG," App crashes on empty config (nil deref on load)") ],
 line(GRAY,"  · acme/widgets · https://github.com/acme/widgets/issues/482"),
 [ (GREEN,"[#903 · score 71]"),(FG," Logs leak the auth token on failed login") ],
 line(GRAY,"  · possible duplicate: #910 \"fix(auth): redact token in failure logs\""),
 [ (BG,"") ],
 line(RED,"### Excluded - already addressed (2)"),
 line(GRAY,"- acme/gadgets#151 Slow pagination from a missing index - PR #168 references it"),
 line(GRAY,"- acme/widgets#129 Stale cache after delete - PR #147 references it"),
 [ (BG,"") ],
 line(FG,"_12 issue(s) examined, 4 ready._"),
]
render("assets/session.png", session, fs=19, pad=26, tab_h=46, title="gh-scout · contribution issues without duplicate work")