using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Threading;
using System.Web.Script.Serialization;

// Runs ONLY under the SDK-created identity, in a different Windows session.
// Compatible with the Windows .NET Framework compiler; no .NET SDK required.
class GuiSmoke
{
    [StructLayout(LayoutKind.Sequential)] struct Keyboard { public ushort Vk, Scan; public uint Flags, Time; public UIntPtr Extra; }
    // Including the mouse member gives INPUT its native union size/alignment.
    [StructLayout(LayoutKind.Sequential)] struct Mouse { public int X, Y; public uint Data, Flags, Time; public UIntPtr Extra; }
    [StructLayout(LayoutKind.Explicit)] struct InputData { [FieldOffset(0)] public Keyboard Key; [FieldOffset(0)] public Mouse Mouse; }
    [StructLayout(LayoutKind.Sequential)] struct Input { public uint Type; public InputData Data; }
    delegate bool WindowCallback(IntPtr window, IntPtr data);
    [DllImport("user32.dll")] static extern bool EnumWindows(WindowCallback callback, IntPtr data);
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr window, out uint pid);
    [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr window);
    [DllImport("user32.dll")] static extern bool SetForegroundWindow(IntPtr window);
    [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll", SetLastError = true)] static extern uint SendInput(uint count, Input[] inputs, int size);

    static readonly JavaScriptSerializer Json = new JavaScriptSerializer();
    static readonly Dictionary<string, object> Report = new Dictionary<string, object>();
    static readonly List<object> Steps = new List<object>();
    static string output, fixture;
    static int sequence;
    static Process app;
    static IntPtr window;
    static string previousImageHash;

    static void Save() { File.WriteAllText(Path.Combine(output, "result.json"), Json.Serialize(Report)); }
    static void Require(bool condition, string message) { if (!condition) throw new InvalidOperationException(message); }
    static bool SamePath(string actual, string expected) { return String.Equals(Path.GetFullPath(actual), Path.GetFullPath(expected), StringComparison.OrdinalIgnoreCase); }

    static IntPtr FindWindow()
    {
        var windows = new List<IntPtr>();
        EnumWindows(delegate(IntPtr candidate, IntPtr ignored) {
            uint owner; GetWindowThreadProcessId(candidate, out owner);
            if (owner == app.Id && IsWindowVisible(candidate)) windows.Add(candidate);
            return true;
        }, IntPtr.Zero);
        Require(windows.Count <= 1, "Expected one NMF window");
        return windows.Count == 1 ? windows[0] : IntPtr.Zero;
    }

    static Input Key(ushort scan, bool up, bool extended)
    {
        return new Input { Type = 1, Data = new InputData { Key = new Keyboard {
            Scan = scan, Flags = 0x0008u | (up ? 0x0002u : 0u) | (extended ? 0x0001u : 0u)
        } } };
    }

    static void Press(ushort scan, bool extended, bool control)
    {
        // Recheck the target before every injection. Never fall back to the host.
        Require(!app.HasExited && app.SessionId == Process.GetCurrentProcess().SessionId, "NMF exited or changed session");
        Require(GetForegroundWindow() == window, "NMF lost foreground in the isolated session");
        uint owner; GetWindowThreadProcessId(window, out owner);
        Require(owner == app.Id, "Input window no longer belongs to NMF");
        var inputs = new List<Input>();
        if (control) inputs.Add(Key(0x1d, false, false));
        inputs.Add(Key(scan, false, extended)); inputs.Add(Key(scan, true, extended));
        if (control) inputs.Add(Key(0x1d, true, false));
        uint sent = SendInput((uint)inputs.Count, inputs.ToArray(), Marshal.SizeOf(typeof(Input)));
        Require(sent == inputs.Count, "SendInput was incomplete: " + sent + "/" + inputs.Count + ", error=" + Marshal.GetLastWin32Error());
        Thread.Sleep(250); // Let the native event loop paint before reading GL_FRONT.
    }

    static Dictionary<string, object> Capture(out string basename)
    {
        Require(!app.HasExited, "NMF exited before capture");
        basename = "capture-" + (++sequence).ToString("D6");
        File.WriteAllText(Path.Combine(output, basename + ".request"), "capture");
        string statePath = Path.Combine(output, basename + ".json");
        var timer = Stopwatch.StartNew();
        while (!File.Exists(statePath) && timer.ElapsedMilliseconds < 5000 && !app.HasExited) Thread.Sleep(50);
        Require(File.Exists(statePath), "Native canvas capture timed out: " + basename);
        var state = Json.Deserialize<Dictionary<string, object>>(File.ReadAllText(statePath));
        Require(!state.ContainsKey("error"), "Canvas capture failed: " + Json.Serialize(state));
        return state;
    }

    static void CheckImage(string path)
    {
        using (var bitmap = new Bitmap(path)) {
            Require(bitmap.Width >= 600 && bitmap.Height >= 400, "Unexpected canvas dimensions");
            var colors = new HashSet<int>();
            // A black client area, missing PNG, or uniform framebuffer must fail.
            for (int y = 0; y < bitmap.Height; y += 3)
                for (int x = 0; x < bitmap.Width; x += 3) colors.Add(bitmap.GetPixel(x, y).ToArgb());
            Require(colors.Count > 8, "Canvas is blank or nearly uniform");
        }
    }

    static void Expect(string name, string relativePath, int count, int cursor, string cursorName, string handler)
    {
        var timer = Stopwatch.StartNew();
        Dictionary<string, object> state;
        string basename;
        bool matches;
        do {
            state = Capture(out basename);
            matches = Convert.ToInt32(state["cursor"]) == cursor && Convert.ToInt32(state["count"]) == count
                && SamePath((string)state["path"], Path.Combine(fixture, relativePath))
                && SamePath((string)state["cursorPath"], Path.Combine(fixture, relativePath, cursorName))
                && (string)state["handler"] == handler;
            if (matches) break;
            Thread.Sleep(150);
        } while (timer.ElapsedMilliseconds < 10000);
        // Preserve the last state/image under a stable name on failure as well.
        File.Copy(Path.Combine(output, basename + ".png"), Path.Combine(output, name + ".png"));
        File.Copy(Path.Combine(output, basename + ".json"), Path.Combine(output, name + ".json"));
        string imagePath = Path.Combine(output, name + ".png");
        string imageHash;
        using (var hash = SHA256.Create())
        using (var stream = File.OpenRead(imagePath)) imageHash = BitConverter.ToString(hash.ComputeHash(stream));
        bool changed = previousImageHash == null || imageHash != previousImageHash;
        Steps.Add(new { name = name, passed = matches && changed, state = state, imageSHA256 = imageHash }); Save();
        Require(matches, "Unexpected state at " + name + ": " + Json.Serialize(state));
        CheckImage(imagePath);
        Require(changed, "Canvas did not change after input at " + name);
        previousImageHash = imageHash;
        Console.WriteLine("PASS " + name);
    }

    static int Main(string[] args)
    {
        if (args.Length != 2) return 64;
        int hostSession;
        int ownSession = Process.GetCurrentProcess().SessionId;
        // Refuse before launching NMF or calling any desktop API.
        if (!Int32.TryParse(args[0], out hostSession) || hostSession <= 0 || ownSession == 0 || ownSession == hostSession
            || !String.Equals(Environment.UserName, args[1], StringComparison.OrdinalIgnoreCase)) {
            Console.Error.WriteLine("Refusing GUI test outside the expected isolated identity/session."); return 65;
        }
        string root = AppDomain.CurrentDomain.BaseDirectory;
        output = Path.Combine(root, "out"); fixture = Path.Combine(root, "files"); Directory.CreateDirectory(output);
        Report["hostSession"] = hostSession; Report["testSession"] = ownSession;
        Report["user"] = Environment.UserName; Report["steps"] = Steps; Report["passed"] = false;
        System.Threading.Tasks.Task<string> stdout = null, stderr = null;
        try {
            var start = new ProcessStartInfo(Path.Combine(root, "nmf-gui.exe"));
            start.Arguments = "-profile \"" + Path.Combine(root, "profile") + "\" -path \"" + fixture + "\" -debug-log \"" + Path.Combine(output, "nmf-debug.log") + "\"";
            start.WorkingDirectory = root; start.UseShellExecute = false; start.CreateNoWindow = true;
            start.RedirectStandardOutput = true; start.RedirectStandardError = true;
            start.EnvironmentVariables["NMF_GUI_TEST_DIR"] = output;
            app = Process.Start(start); stdout = app.StandardOutput.ReadToEndAsync(); stderr = app.StandardError.ReadToEndAsync();
            Report["nmfSession"] = app.SessionId; Report["nmfPid"] = app.Id;
            Require(app.SessionId == ownSession, "NMF started outside the isolated session");
            var timer = Stopwatch.StartNew();
            while (!app.HasExited && timer.ElapsedMilliseconds < 15000) {
                window = FindWindow(); if (window != IntPtr.Zero) break; Thread.Sleep(100);
            }
            Require(window != IntPtr.Zero && !app.HasExited, "NMF did not create a visible window");
            SetForegroundWindow(window); Thread.Sleep(300);
            Require(GetForegroundWindow() == window, "Could not focus isolated NMF");
            Expect("01-startup", ".", 4, 0, "..", "MainScreen");
            Press(0x50, true, false); // Down (extended scan code)
            Expect("02-cursor-down", ".", 4, 1, "folder-a", "MainScreen");
            Press(0x21, false, true); // Ctrl+F
            Expect("03-filter-open", ".", 4, 1, "folder-a", "FilterDialog");
            Press(0x01, false, false); // Escape
            Expect("04-filter-closed", ".", 4, 1, "folder-a", "MainScreen");
            Press(0x1c, false, false); // Enter
            Expect("05-enter-directory", "folder-a", 2, 0, "..", "MainScreen");
            Press(0x0e, false, false); // Backspace
            Expect("06-parent-directory", ".", 4, 1, "folder-a", "MainScreen");
            Report["passed"] = true;
        } catch (Exception error) {
            Report["error"] = error.ToString(); Console.Error.WriteLine(error.Message);
        } finally {
            // This process owns only the disposable test instance and fixture.
            if (app != null) {
                try { if (!app.HasExited) { app.Kill(); Require(app.WaitForExit(5000), "NMF did not terminate"); } }
                catch (Exception error) { Report["cleanupError"] = error.Message; Report["passed"] = false; }
                if (stdout != null && stdout.Wait(2000)) File.WriteAllText(Path.Combine(output, "stdout.log"), stdout.Result);
                if (stderr != null && stderr.Wait(2000)) File.WriteAllText(Path.Combine(output, "stderr.log"), stderr.Result);
                app.Dispose();
            }
            Save();
        }
        return (bool)Report["passed"] ? 0 : 1;
    }
}
