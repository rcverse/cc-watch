#!/usr/bin/env python3
"""
cc-cache — Claude Code cache TTL inspector

Usage:
  cc-cache              List 5 most recent sessions
  cc-cache --n 10       List N most recent sessions
  cc-cache --id <id>    Inspect a specific session (partial UUID OK)
"""

import argparse
import collections
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

# ── ANSI colours ────────────────────────────────────────────────────────────

RESET  = "\033[0m"
BOLD   = "\033[1m"
DIM    = "\033[2m"
RED    = "\033[38;5;203m"
GREEN  = "\033[38;5;114m"
YELLOW = "\033[38;5;221m"
CYAN   = "\033[38;5;117m"
WHITE  = "\033[38;5;255m"
GREY   = "\033[38;5;245m"

BG_RED   = "\033[48;5;52m"
BG_GREEN = "\033[48;5;22m"

def b(s):   return f"{BOLD}{s}{RESET}"
def dim(s): return f"{DIM}{GREY}{s}{RESET}"
def ok(s):  return f"{GREEN}{s}{RESET}"
def warn(s):return f"{YELLOW}{s}{RESET}"
def err(s): return f"{RED}{s}{RESET}"
def hi(s):  return f"{CYAN}{s}{RESET}"

# ── Progress bar ─────────────────────────────────────────────────────────────

def colour_bar(pct: float, width: int = 24) -> str:
    """Filled block bar, colour shifts green→yellow→red as pct increases."""
    filled = round(pct / 100 * width)
    filled = max(0, min(width, filled))
    empty  = width - filled

    if pct < 50:
        colour = GREEN
    elif pct < 80:
        colour = YELLOW
    else:
        colour = RED

    return f"{colour}{'█' * filled}{RESET}{DIM}{'░' * empty}{RESET}"

# ── Data loading ─────────────────────────────────────────────────────────────

PROJECTS_DIR = Path.home() / ".claude" / "projects"

def find_sessions(n: int) -> list[Path]:
    """Return the N most recently modified session JSONL files."""
    if not PROJECTS_DIR.exists():
        sys.exit(err(f"Directory not found: {PROJECTS_DIR}\n"
                     "Has Claude Code been run on this machine?"))
    files = list(PROJECTS_DIR.rglob("*.jsonl"))
    files.sort(key=lambda p: p.stat().st_mtime, reverse=True)
    return files[:n]

def find_session_by_id(partial_id: str) -> Path:
    """Find a session file whose name contains the given ID fragment."""
    if not PROJECTS_DIR.exists():
        sys.exit(err(f"Directory not found: {PROJECTS_DIR}"))
    for f in PROJECTS_DIR.rglob("*.jsonl"):
        if partial_id.lower() in f.stem.lower():
            return f
    sys.exit(err(f"No session matching '{partial_id}' found under {PROJECTS_DIR}"))

def parse_session(path: Path) -> dict:
    """Parse a session JSONL into a structured data dict."""
    totals     = collections.defaultdict(int)
    timestamps = []
    user_msgs  = []

    with open(path, encoding="utf-8", errors="replace") as fh:
        for raw in fh:
            raw = raw.strip()
            if not raw:
                continue
            try:
                obj = json.loads(raw)
            except json.JSONDecodeError:
                continue

            # Timestamp
            ts = obj.get("timestamp")
            if ts:
                try:
                    timestamps.append(
                        datetime.fromisoformat(ts.replace("Z", "+00:00"))
                    )
                except ValueError:
                    pass

            # Token usage — flat + one level of nested dicts
            usage = obj.get("usage") or obj.get("message", {}).get("usage", {})
            if isinstance(usage, dict):
                for k, v in usage.items():
                    if isinstance(v, (int, float)):
                        totals[k] += v
                    elif isinstance(v, dict):
                        for k2, v2 in v.items():
                            if isinstance(v2, (int, float)):
                                totals[k2] += v2

            # User messages
            msg = obj.get("message", {})
            if isinstance(msg, dict) and msg.get("role") == "user":
                content = msg.get("content", "")
                text = ""
                if isinstance(content, str):
                    text = content.strip()
                elif isinstance(content, list):
                    for block in content:
                        if isinstance(block, dict) and block.get("type") == "text":
                            text = block.get("text", "").strip()
                            break
                if text:
                    user_msgs.append(text)

    timestamps.sort()
    now    = datetime.now(timezone.utc)
    ttl_1h = totals.get("ephemeral_1h_input_tokens", 0)
    ttl_5m = totals.get("ephemeral_5m_input_tokens", 0)

    if ttl_1h > 0:
        tier, ttl_s = "1h", 3600
    elif ttl_5m > 0:
        tier, ttl_s = "5m", 300
    else:
        tier, ttl_s = "?", 300   # unknown, assume 5m for gap detection

    last_ts    = timestamps[-1] if timestamps else None
    elapsed_s  = (now - last_ts).total_seconds() if last_ts else None
    remaining_s = (ttl_s - elapsed_s) if elapsed_s is not None else None
    expired    = (elapsed_s > ttl_s) if elapsed_s is not None else None

    # Gap analysis
    gaps = []
    for i in range(1, len(timestamps)):
        gap_s = (timestamps[i] - timestamps[i - 1]).total_seconds()
        if gap_s > 60:
            gaps.append((gap_s, timestamps[i - 1], timestamps[i]))
    gaps.sort(reverse=True)

    cache_create = totals.get("cache_creation_input_tokens", 0)
    cache_read   = totals.get("cache_read_input_tokens", 0)
    output_tok   = totals.get("output_tokens", 0)
    hit_rate     = (
        cache_read / (cache_read + cache_create) * 100
        if (cache_read + cache_create) > 0 else 0
    )

    # Project name from directory
    project_raw = path.parent.name
    parts = project_raw.lstrip("-").split("-")
    project = parts[-1] if parts else project_raw

    return {
        "session_id":  path.stem,
        "project":     project,
        "tier":        tier,
        "ttl_s":       ttl_s,
        "elapsed_s":   elapsed_s,
        "remaining_s": remaining_s,
        "expired":     expired,
        "last_ts":     last_ts,
        "gaps":        gaps,
        "cache_create":cache_create,
        "cache_read":  cache_read,
        "hit_rate":    hit_rate,
        "output_tok":  output_tok,
        "first_msg":   user_msgs[0]  if user_msgs else None,
        "last_msg":    user_msgs[-1] if user_msgs else None,
    }

# ── Formatting helpers ────────────────────────────────────────────────────────

def fmt_elapsed(s: float) -> str:
    s = abs(s)
    h = int(s // 3600)
    m = int((s % 3600) // 60)
    sec = int(s % 60)
    if h:
        return f"{h}h {m:02d}m"
    if m:
        return f"{m}m {sec:02d}s"
    return f"{sec}s"

def status_line(d: dict) -> str:
    if d["expired"] is None:
        return dim("no timestamps")
    elif d["expired"]:
        t = fmt_elapsed(d["elapsed_s"])
        return err(f"✗ EXPIRED") + dim(f"  {t} ago")
    else:
        t = fmt_elapsed(d["remaining_s"])
        return ok(f"✓ ACTIVE") + dim(f"  {t} remaining")

def tier_badge(tier: str) -> str:
    if tier == "1h":
        return hi("🕐 1-HOUR") + dim("  (subscription)")
    elif tier == "5m":
        return warn("⏱  5-MIN") + dim("  (API default)")
    return dim("? unknown")

def trunc(s: str, n: int = 72) -> str:
    if not s:
        return dim("(none)")
    s = s.replace("\n", " ")
    return (s[:n] + dim("…")) if len(s) > n else s

# ── Card view (list mode: one card per session) ───────────────────────────────

CARD_W = 62

def print_card(d: dict, index: int) -> None:
    sid      = d["session_id"]
    sid_short = sid[:8]
    pct_elapsed = (
        min(d["elapsed_s"] / d["ttl_s"] * 100, 100)
        if d["elapsed_s"] is not None else 0
    )
    bar = colour_bar(pct_elapsed)
    reset_count = sum(1 for g, _, _ in d["gaps"] if g > d["ttl_s"])

    # Header row
    idx_str  = dim(f"#{index}")
    sid_str  = b(sid_short) + dim(f"…{sid[8:13]}")
    proj_str = CYAN + d["project"] + RESET
    tier_str = tier_badge(d["tier"])

    print(f"  {idx_str}  {sid_str}  {dim('·')}  {proj_str}  {dim('·')}  {tier_str}")
    print(f"     {status_line(d)}  {dim('│')}  {bar}")

    # Messages
    first = ("  " + dim("first") + f"  {trunc(d['first_msg'], 60)}") if d["first_msg"] else ""
    last  = ("  " + dim("last ") + f"  {trunc(d['last_msg'],  60)}") if d["last_msg"]  else ""
    if first: print(first)
    if last:  print(last)

    # Gap warning (only if resets detected)
    if reset_count:
        print(f"     {warn(f'⚠  {reset_count} cache reset(s) mid-session')}")

    print()

# ── Detail view (--id mode) ──────────────────────────────────────────────────

def print_detail(d: dict) -> None:
    sid = d["session_id"]
    W = 59

    TL, TR, BL, BR = "╭", "╮", "╰", "╯"
    H, V, X = "─", "│", "┼"

    def box_line(content: str) -> str:
        # strip ANSI for width measurement
        import re
        plain = re.sub(r"\033\[[0-9;]*m", "", content)
        pad = W - len(plain) - 2
        return f"{V}  {content}{' ' * max(pad, 0)}  {V}"

    print()
    print(TL + H * (W + 2) + TR)
    print(box_line(b("Claude Code Cache Inspector")))
    print(box_line(dim("Session: ") + CYAN + sid + RESET))
    print(box_line(dim("Project: ") + d["project"]))
    print(BL + H * (W + 2) + BR)
    print()

    pct_elapsed = (
        min(d["elapsed_s"] / d["ttl_s"] * 100, 100)
        if d["elapsed_s"] is not None else 0
    )
    bar = colour_bar(pct_elapsed)

    print(f"  {b('TTL Tier')}     {V}  {tier_badge(d['tier'])}")
    print(f"  {H*13}{X}{H*44}")
    print(f"  {b('Status')}       {V}  {status_line(d)}")
    print(f"               {V}  {bar}  {dim(f'{pct_elapsed:.0f}% elapsed')}")
    if d["last_ts"]:
        last_local = d["last_ts"].astimezone().strftime("%H:%M:%S %Z")
        print(f"  {dim('Last msg')}     {V}  {dim(last_local)}  "
              f"{dim('(' + fmt_elapsed(d['elapsed_s']) + ' ago)')}")
    print()

    # Token stats
    cr  = d["cache_read"]
    cc  = d["cache_create"]
    out = d["output_tok"]
    hr  = d["hit_rate"]

    print(f"  {dim('── Token Stats ') + DIM + '─'*44 + RESET}")
    print(f"  {'Cache writes':<15} {V}  {WHITE}{cc:>13,}{RESET}  {dim('tokens (creation)')}")
    print(f"  {'Cache reads':<15} {V}  {WHITE}{cr:>13,}{RESET}  {dim('tokens (from cache)')}")
    print(f"  {'Hit rate':<15} {V}  {colour_bar(hr)}  {GREEN if hr >= 80 else YELLOW}{hr:.0f}%{RESET}")
    print(f"  {'Output':<15} {V}  {WHITE}{out:>13,}{RESET}  {dim('tokens')}")
    print()

    # Gap analysis
    gaps = d["gaps"]
    ttl_s = d["ttl_s"]
    reset_count = sum(1 for g, _, _ in gaps if g > ttl_s)

    print(f"  {dim('── Mid-session Gaps >1min ') + DIM + '─'*33 + RESET}")
    if gaps:
        for i, (gap_s, t1, t2) in enumerate(gaps[:5]):
            is_reset = gap_s > ttl_s
            icon     = warn("⚠") + " " + dim("CACHE RESET") if is_reset else ok("✓") + "           "
            note     = dim("  ← longest") if i == 0 else ""
            t1s      = t1.astimezone().strftime("%H:%M:%S")
            t2s      = t2.astimezone().strftime("%H:%M:%S")
            gap_fmt  = f"{int(gap_s//60):>3}m {int(gap_s%60):02d}s"
            print(f"  {icon}  {CYAN}{gap_fmt}{RESET}   {dim(t1s)} {dim('→')} {dim(t2s)}{note}")
        print()
        if reset_count == 0:
            print(f"  {ok('✓')} {dim('No cache resets detected during this session.')}")
        else:
            print(f"  {warn('⚠')}  {warn(str(reset_count) + ' cache reset(s)')} "
                  f"{dim('— rebuilt from scratch ' + str(reset_count) + ' time(s).')}")
    else:
        print(f"  {dim('No significant gaps found.')}")
    print()

# ── List header ───────────────────────────────────────────────────────────────

def print_list_header(n: int) -> None:
    now_str = datetime.now().strftime("%H:%M:%S")
    print()
    print(f"  {b('Claude Code Cache')}  {dim('·')}  {dim(f'{n} most recent sessions')}  "
          f"{dim('·')}  {dim(now_str)}")
    print(f"  {DIM}{'─' * 60}{RESET}")
    print()

# ── Main ─────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        prog="cc-cache",
        description="Inspect Claude Code prompt cache TTL and status",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Examples:\n"
            "  cc-cache              # list 5 most recent sessions\n"
            "  cc-cache --n 10       # list 10 most recent\n"
            "  cc-cache --id d4b247b7  # inspect a session (partial UUID OK)\n"
        ),
    )
    parser.add_argument("--id", metavar="SESSION_ID",
                        help="Inspect a specific session (partial UUID OK)")
    parser.add_argument("--n", metavar="N", type=int, default=5,
                        help="Number of recent sessions to list (default: 5)")
    args = parser.parse_args()

    if args.id:
        path = find_session_by_id(args.id)
        d    = parse_session(path)
        print_detail(d)
    else:
        paths = find_sessions(args.n)
        if not paths:
            print(err("No Claude Code sessions found."))
            sys.exit(1)
        print_list_header(len(paths))
        for i, path in enumerate(paths, 1):
            try:
                d = parse_session(path)
                print_card(d, i)
            except Exception as exc:
                print(f"  {dim(f'#{i}')}  {err(f'Failed to parse {path.name}: {exc}')}\n")

if __name__ == "__main__":
    main()
