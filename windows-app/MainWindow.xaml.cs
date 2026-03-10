using System;
using System.Text.Json;
using System.Windows;
using System.Windows.Input;
using Microsoft.Web.WebView2.Core;

namespace Forumline;

public partial class MainWindow : Window
{
    private static readonly HashSet<string> AllowedHosts = new(StringComparer.OrdinalIgnoreCase)
    {
        "app.forumline.net",
        "demo.forumline.net",
        "forumline.net",
    };

    public MainWindow()
    {
        InitializeComponent();
        InitializeWebView();
    }

    private async void InitializeWebView()
    {
        await webView.EnsureCoreWebView2Async();
        var coreWebView = webView.CoreWebView2;

        // Inject the native bridge JS before any page loads
        await coreWebView.AddScriptToExecuteOnDocumentCreatedAsync(BridgeJs);

        // Handle web → native messages
        coreWebView.WebMessageReceived += OnWebMessageReceived;

        // URL whitelist: only allow forumline.net domains
        coreWebView.NavigationStarting += OnNavigationStarting;

        // Store bridge reference
        WebViewBridge.Instance.CoreWebView = coreWebView;

        // Disable dev tools in release builds
#if !DEBUG
        coreWebView.Settings.AreDevToolsEnabled = false;
#endif

        // Allow microphone for WebRTC voice calls
        coreWebView.PermissionRequested += (_, args) =>
        {
            if (args.PermissionKind == CoreWebView2PermissionKind.Microphone)
            {
                args.State = CoreWebView2PermissionState.Allow;
            }
        };
    }

    private void OnNavigationStarting(object? sender, CoreWebView2NavigationStartingEventArgs e)
    {
        if (!Uri.TryCreate(e.Uri, UriKind.Absolute, out var uri))
        {
            e.Cancel = true;
            return;
        }

        // Allow internal schemes
        if (uri.Scheme is "about" or "blob" or "data")
            return;

        var host = uri.Host;

        // Allow forumline.net domains
        if (AllowedHosts.Contains(host) || host.EndsWith(".forumline.net", StringComparison.OrdinalIgnoreCase))
            return;

        // Open external links in the default browser
        e.Cancel = true;
        try
        {
            var psi = new System.Diagnostics.ProcessStartInfo(e.Uri)
            {
                UseShellExecute = true
            };
            System.Diagnostics.Process.Start(psi);
        }
        catch { }
    }

    private void OnWebMessageReceived(object? sender, CoreWebView2WebMessageReceivedEventArgs e)
    {
        try
        {
            var message = e.WebMessageAsJson;
            using var doc = JsonDocument.Parse(message);
            var root = doc.RootElement;

            if (!root.TryGetProperty("type", out var typeProp))
                return;

            var type = typeProp.GetString();
            switch (type)
            {
                case "auth-state":
                    HandleAuthState(root);
                    break;
                default:
                    System.Diagnostics.Debug.WriteLine($"[Forumline] Unknown bridge message: {type}");
                    break;
            }
        }
        catch (Exception ex)
        {
            System.Diagnostics.Debug.WriteLine($"[Forumline] Bridge message error: {ex.Message}");
        }
    }

    private void HandleAuthState(JsonElement json)
    {
        string? token = null;
        if (json.TryGetProperty("accessToken", out var tokenProp))
            token = tokenProp.GetString();

        // Token available for future push notification registration
        System.Diagnostics.Debug.WriteLine(
            token != null ? "[Forumline] User authenticated" : "[Forumline] User logged out");
    }

    /// <summary>
    /// Bridge JS injected before page load. Sets up the same interface
    /// the web app expects, routing through WebView2's postMessage.
    /// </summary>
    private const string BridgeJs = """
        (function() {
            if (window.__FORUMLINE_WINDOWS__) return;
            window.__FORUMLINE_WINDOWS__ = true;

            window.forumlineNative = {
                postMessage: function(msg) {
                    window.chrome.webview.postMessage(
                        typeof msg === 'string' ? JSON.parse(msg) : msg
                    );
                }
            };

            window.forumlineNativeBridge = {
                _handlers: [],
                onMessage: function(msg) {
                    for (var i = 0; i < this._handlers.length; i++) {
                        try { this._handlers[i](msg); } catch(e) { console.error('[NativeBridge]', e); }
                    }
                },
                addHandler: function(fn) {
                    this._handlers.push(fn);
                }
            };
        })();
        """;
}
