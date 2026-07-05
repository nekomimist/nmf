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
           "Escape", "BackSpace", "Delete", "Tab", "F1".."F12"), a single
           character ("q", "r"), with optional modifier prefixes "S-"
           (Shift), "C-" (Control), "A-" (Alt), combinable as "C-S-x".
Examples:
  uv run inject_keys.py "File Manager" Down 200 5     # hold-down simulation
  uv run inject_keys.py "File Manager" S-Period 1 50  # cursor to last entry
  uv run inject_keys.py "File Manager" C-F 1 50       # Ctrl+F
"""

import sys
import time

from Xlib import X, XK, display, protocol

NAMED = {
    "Down": XK.XK_Down,
    "Up": XK.XK_Up,
    "Left": XK.XK_Left,
    "Right": XK.XK_Right,
    "Return": XK.XK_Return,
    "space": XK.XK_space,
    "Period": XK.XK_period,
    "Comma": XK.XK_comma,
    "Escape": XK.XK_Escape,
    "BackSpace": XK.XK_BackSpace,
    "Delete": XK.XK_Delete,
    "Tab": XK.XK_Tab,
}
NAMED.update({f"F{i}": getattr(XK, f"XK_F{i}") for i in range(1, 13)})

MODIFIERS = {
    "S": X.ShiftMask,
    "C": X.ControlMask,
    "A": X.Mod1Mask,
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
    keyname = keyspec
    state = 0
    while len(keyname) > 2 and keyname[1] == "-" and keyname[0] in MODIFIERS:
        state |= MODIFIERS[keyname[0]]
        keyname = keyname[2:]
    d = display.Display()
    root = d.screen().root
    win = find_window(d, root, title)
    if win is None:
        print(f"ERROR: window containing '{title}' not found", file=sys.stderr)
        sys.exit(1)
    keycode = d.keysym_to_keycode(keysym_for(keyname))
    mod_keycodes = [
        d.keysym_to_keycode(sym)
        for mask, sym in (
            (X.ShiftMask, XK.XK_Shift_L),
            (X.ControlMask, XK.XK_Control_L),
            (X.Mod1Mask, XK.XK_Alt_L),
        )
        if state & mask
    ]
    for _ in range(count):
        for kc in mod_keycodes:
            send_key(d, root, win, kc, True)
        send_key(d, root, win, keycode, True, state)
        send_key(d, root, win, keycode, False, state)
        for kc in reversed(mod_keycodes):
            send_key(d, root, win, kc, False)
        d.flush()
        if delay_ms > 0:
            time.sleep(delay_ms / 1000.0)
    d.sync()
    print(f"sent {count}x {keyspec}")


if __name__ == "__main__":
    main()
