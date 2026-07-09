# /// script
# requires-python = ">=3.9"
# dependencies = ["python-xlib"]
# ///
"""Nudge an X11 window to force a repaint: resize by +1px, send Expose, resize back.

Usage: uv run poke_window.py <title-substr>
"""

import sys
import time

from Xlib import X, display, protocol


def find_window(d, root, title_substr):
    stack = [root]
    while stack:
        w = stack.pop()
        try:
            name = w.get_wm_name()
        except Exception:
            name = None
        if name and title_substr in name:
            return w
        try:
            stack.extend(w.query_tree().children)
        except Exception:
            pass
    return None


def main():
    title = sys.argv[1]
    d = display.Display()
    root = d.screen().root
    win = find_window(d, root, title)
    if win is None:
        print(f"ERROR: window containing '{title}' not found", file=sys.stderr)
        sys.exit(1)
    geom = win.get_geometry()
    print(f"found window id=0x{win.id:x} {geom.width}x{geom.height}")

    ev = protocol.event.Expose(
        window=win, x=0, y=0,
        width=geom.width, height=geom.height, count=0,
    )
    win.send_event(ev, propagate=False)
    d.flush()
    time.sleep(0.3)

    win.configure(width=geom.width + 1, height=geom.height)
    d.flush()
    time.sleep(0.5)
    win.configure(width=geom.width, height=geom.height)
    d.flush()
    d.sync()
    print("poked: expose + resize jiggle")


if __name__ == "__main__":
    main()
