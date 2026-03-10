import SwiftUI
import WebKit

struct WebViewContainer: NSViewRepresentable {
    func makeNSView(context: Context) -> WKWebView {
        let config = WKWebViewConfiguration()

        // Allow inline media playback (critical for WebRTC audio)
        config.mediaTypesRequiringUserActionForPlayback = []

        // Inject the native bridge script at document start
        let bridgeScript = WKUserScript(
            source: Self.bridgeJS,
            injectionTime: .atDocumentStart,
            forMainFrameOnly: true
        )
        config.userContentController.addUserScript(bridgeScript)

        // Register the message handler for web → native communication
        config.userContentController.add(
            context.coordinator,
            name: "forumlineNative"
        )

        // Microphone access for WebRTC
        config.preferences.setValue(true, forKey: "mediaDevicesEnabled")
        config.preferences.setValue(true, forKey: "mediaCaptureRequiresSecureConnection")

        let webView = WKWebView(frame: .zero, configuration: config)

        // Match the app background color (#0f172a)
        webView.setValue(false, forKey: "drawsBackground")
        webView.wantsLayer = true
        webView.layer?.backgroundColor = NSColor(
            red: 15/255, green: 23/255, blue: 42/255, alpha: 1
        ).cgColor

        webView.allowsBackForwardNavigationGestures = true
        webView.navigationDelegate = context.coordinator

        // Store reference for native → web messaging
        WebViewBridge.shared.webView = webView

        // To test locally: change to "http://localhost:3001"
        let url = URL(string: "https://app.forumline.net")!
        webView.load(URLRequest(url: url))

        return webView
    }

    func updateNSView(_ nsView: WKWebView, context: Context) {}

    func makeCoordinator() -> WebViewCoordinator {
        WebViewCoordinator()
    }

    /// JavaScript injected at document start to set up the native bridge
    private static let bridgeJS = """
    window.__FORUMLINE_MACOS__ = true;
    window.forumlineNative = {
        postMessage: function(msg) {
            window.webkit.messageHandlers.forumlineNative.postMessage(
                typeof msg === 'string' ? msg : JSON.stringify(msg)
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
    """
}

/// Handles messages from web → native
class WebViewCoordinator: NSObject, WKScriptMessageHandler, WKNavigationDelegate {
    private static let allowedHosts: Set<String> = [
        "app.forumline.net",
        "demo.forumline.net",
        "forumline.net",
    ]

    func webView(
        _ webView: WKWebView,
        decidePolicyFor navigationAction: WKNavigationAction,
        decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
    ) {
        guard let url = navigationAction.request.url else {
            decisionHandler(.cancel)
            return
        }

        // Allow about:blank and blob: URLs (used internally by WebKit)
        if url.scheme == "about" || url.scheme == "blob" {
            decisionHandler(.allow)
            return
        }

        // Allow navigation only to forumline.net domains
        if let host = url.host, Self.allowedHosts.contains(host) || host.hasSuffix(".forumline.net") {
            decisionHandler(.allow)
            return
        }

        // Open external links in the default browser
        if let host = url.host {
            print("[Forumline] Blocked navigation to disallowed host: \(host)")
        }
        if url.scheme == "https" || url.scheme == "http" {
            NSWorkspace.shared.open(url)
        }
        decisionHandler(.cancel)
    }

    func userContentController(
        _ userContentController: WKUserContentController,
        didReceive message: WKScriptMessage
    ) {
        // Validate the message originates from an allowed domain
        if let url = message.frameInfo.request.url,
           let host = url.host,
           !Self.allowedHosts.contains(host) && !host.hasSuffix(".forumline.net") {
            print("[Forumline] Rejected bridge message from disallowed origin: \(host)")
            return
        }

        guard let body = message.body as? String,
              let data = body.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = json["type"] as? String
        else { return }

        switch type {
        case "auth-state":
            handleAuthState(json)
        default:
            print("[Forumline] Unknown bridge message: \(type)")
        }
    }

    private func handleAuthState(_ json: [String: Any]) {
        let token = json["accessToken"] as? String
        PushManager.shared.accessToken = token
        if token != nil {
            PushManager.shared.registerTokensWithServer()
        }
    }
}
