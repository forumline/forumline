using System.Text.Json;
using Microsoft.Web.WebView2.Core;

namespace Forumline;

/// <summary>
/// Singleton for native → web messaging.
/// Windows equivalent of iOS/Android/macOS WebViewBridge.
/// </summary>
public sealed class WebViewBridge
{
    public static WebViewBridge Instance { get; } = new();

    public CoreWebView2? CoreWebView { get; set; }

    /// <summary>
    /// Send a message from native to the web app.
    /// </summary>
    public void SendToWeb(object message)
    {
        if (CoreWebView == null) return;

        var json = JsonSerializer.Serialize(message);
        var js = $"window.forumlineNativeBridge?.onMessage({json});";

        System.Windows.Application.Current?.Dispatcher.InvokeAsync(() =>
        {
            CoreWebView.ExecuteScriptAsync(js);
        });
    }
}
