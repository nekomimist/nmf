# /// script
# requires-python = ">=3.9"
# dependencies = ["python-xlib"]
# ///
"""Send synthetic key events to an X11 window via XSendEvent.

Works on WSLg where XTEST/xdotool input is silently dropped (the Wayland
compositor owns focus, but GLFW/Fyne apps process send_event key events
delivered straight to the window).

Usage: uv run inject_keys.py <title-substr> <keyspec> <count> <delay_ms>
  keyspec: named key ("Down", "Up", "Return", "space", "Period", "Comma",
           "Escape", "BackSpace", "F2"), a single character ("q", "r"),
           or a shifted combo ("S-Period", "S-s").
Examples:
  uv run inject_keys.py "File Manager" Down 200 5     # hold-down simulation
  uv run inject_keys.py "File Manager" S-Period 1 50  # cursor to last entry
"""

import sys
import time

from Xlib import X, XK, display, protocol

NAMED = {
    "Down": XK.XK_Down,
    "Up": XK.XK_Up,
    "Return": XK.XK_Return,
    "space": XK.XK_space,
    "Period": XK.XK_period,
    "Comma": XK.XK_comma,
    "Escape": XK.XK_Escape,
    "BackSpace": XK.XK_BackSpace,
    "F2": XK.XK_F2,
}


def keysym_for(name):
    if name in NAMED:
        return NAMED[name]
    if len(name) == 1:
        return XK.string_to_keysym(name)
    raise SystemExit(f"unknown key: {name}")


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


def send_key(d, root, win, keycode, press, state=0):
    cls = protocol.event.KeyPress if press else protocol.event.KeyRelease
    ev = cls(
        time=X.CurrentTime,
        root=root,
        window=win,
        same_screen=1,
        child=X.NONE,
        root_x=0,
        root_y=0,
        event_x=1,
        event_y=1,
        state=state,
        detail=keycode,
    )
    win.send_event(ev, propagate=False)


def main():
    title, keyspec, count, delay_ms = (
        sys.argv[1],
        sys.argv[2],
        int(sys.argv[3]),
        float(sys.argv[4]),
    )
    shift = keyspec.startswith("S-")
    keyname = keyspec[2:] if shift else keyspec
    d = display.Display()
    root = d.screen().root
    win = find_window(d, root, title)
    if win is None:
        print(f"ERROR: window containing '{title}' not found", file=sys.stderr)
        sys.exit(1)
    keycode = d.keysym_to_keycode(keysym_for(keyname))
    shift_kc = d.keysym_to_keycode(XK.XK_Shift_L)
    for _ in range(count):
        if shift:
            send_key(d, root, win, shift_kc, True)
        state = X.ShiftMask if shift else 0
        send_key(d, root, win, keycode, True, state)
        send_key(d, root, win, keycode, False, state)
        if shift:
            send_key(d, root, win, shift_kc, False)
        d.flush()
        if delay_ms > 0:
            time.sleep(delay_ms / 1000.0)
    d.sync()
    print(f"sent {count}x {keyspec}")


if __name__ == "__main__":
    main()
