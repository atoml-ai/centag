#!/usr/bin/env python3
"""Escape vue-i18n special characters in locale JSON files.

vue-i18n v9 treats:
  @  as linked messages
  |  as plural separator
  {{x}} as nested placeholder (error code 9)

This script:
  1) Converts known interpolations {{name}} -> {name}
  2) Escapes remaining {{ / }} that document Go templates as {'{{'} / {'}}'}
  3) Escapes bare @ and | as {'@'} / {'|'}
"""

from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "src" / "locales"

# Named interpolations actually passed to t(key, { ... })
INTERPOLATION_NAMES = {
    "url",
    "n",
    "nodeId",
    "count",
    "name",
    "agent",
    "pipeline",
}


def escape_literals(s: str) -> str:
    """Escape remaining {{ }} @ | that are not already escaped."""
    # Protect already-escaped literals
    protected: list[str] = []

    def protect(m: re.Match[str]) -> str:
        protected.append(m.group(0))
        return f"\x00{len(protected) - 1}\x00"

    s = re.sub(r"\{'(?:@|\||\{\{|\}\})'\}", protect, s)

    # Escape leftover double braces (Go template docs / literal text)
    s = s.replace("{{", "{'{{'}").replace("}}", "{'}}'}")

    # Escape bare @ and |
    s = s.replace("@", "{'@'}").replace("|", "{'|'}")

    # Restore protected
    def restore(m: re.Match[str]) -> str:
        return protected[int(m.group(1))]

    return re.sub(r"\x00(\d+)\x00", restore, s)


def fix_string(s: str) -> str:
    # 1) known vue-i18n interpolations: {{name}} -> {name}
    def repl_interp(m: re.Match[str]) -> str:
        name = m.group(1)
        if name in INTERPOLATION_NAMES:
            return "{" + name + "}"
        return m.group(0)

    s = re.sub(r"\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}", repl_interp, s)

    # 2) escape remaining specials
    return escape_literals(s)


def walk(obj):
    changed = False
    if isinstance(obj, dict):
        for k, v in list(obj.items()):
            nv, c = walk(v)
            if c:
                obj[k] = nv
                changed = True
        return obj, changed
    if isinstance(obj, list):
        for i, v in enumerate(obj):
            nv, c = walk(v)
            if c:
                obj[i] = nv
                changed = True
        return obj, changed
    if isinstance(obj, str):
        ns = fix_string(obj)
        return ns, ns != obj
    return obj, False


def main() -> None:
    files = sorted(ROOT.rglob("*.json"))
    total = 0
    for path in files:
        data = json.loads(path.read_text(encoding="utf-8"))
        data, changed = walk(data)
        if changed:
            path.write_text(
                json.dumps(data, ensure_ascii=False, indent=2) + "\n",
                encoding="utf-8",
            )
            total += 1
            print("updated", path.relative_to(ROOT))
    print(f"done, {total} files changed")


if __name__ == "__main__":
    main()
