#!/usr/bin/env python3
"""Generate web favicons from a custom icon PNG."""
import sys
from pathlib import Path

try:
    from PIL import Image
except ImportError:
    print("错误: 缺少 Pillow 库，请运行: python3 -m pip install Pillow", file=sys.stderr)
    sys.exit(1)

src_path = Path(sys.argv[1])
webui_public = Path(sys.argv[2]) if len(sys.argv) > 2 else Path("web/public")
svg_path = Path(sys.argv[3]) if len(sys.argv) > 3 else Path("web/src/components/icons/centag-mark.svg")

src = Image.open(src_path)

# favicon.png (256x256)
fav = src.resize((256, 256), Image.LANCZOS)
fav.save(webui_public / "favicon.png")

# favicon.ico (multi-size)
ico = src.resize((32, 32), Image.LANCZOS)
ico.save(webui_public / "favicon.ico", format="ICO", sizes=[(32, 32), (16, 16)])

# For SVG: write a simple SVG that embeds the PNG as base64
# This preserves the visual appearance without vector tracing
small = src.resize((128, 128), Image.LANCZOS)
import base64, io
buf = io.BytesIO()
small.save(buf, format="PNG")
b64 = base64.b64encode(buf.getvalue()).decode()
svg_content = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" width="128" height="128">
  <image href="data:image/png;base64,{b64}" width="128" height="128"/>
</svg>
'''
svg_path.parent.mkdir(parents=True, exist_ok=True)
svg_path.write_text(svg_content)

print(f"favicons generated from {src_path}")
