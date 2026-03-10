#include <gtk/gtk.h>
#include <webkit/webkit.h>

static const char *ALLOWED_HOSTS[] = {
    "app.forumline.net",
    "demo.forumline.net",
    "forumline.net",
    NULL
};

static const char *BRIDGE_JS =
    "(function() {\n"
    "    if (window.__FORUMLINE_LINUX__) return;\n"
    "    window.__FORUMLINE_LINUX__ = true;\n"
    "\n"
    "    window.forumlineNative = {\n"
    "        postMessage: function(msg) {\n"
    "            window.webkit.messageHandlers.forumlineNative.postMessage(\n"
    "                typeof msg === 'string' ? msg : JSON.stringify(msg)\n"
    "            );\n"
    "        }\n"
    "    };\n"
    "\n"
    "    window.forumlineNativeBridge = {\n"
    "        _handlers: [],\n"
    "        onMessage: function(msg) {\n"
    "            for (var i = 0; i < this._handlers.length; i++) {\n"
    "                try { this._handlers[i](msg); } catch(e) { console.error('[NativeBridge]', e); }\n"
    "            }\n"
    "        },\n"
    "        addHandler: function(fn) {\n"
    "            this._handlers.push(fn);\n"
    "        }\n"
    "    };\n"
    "})();\n";

static gboolean is_host_allowed(const char *host)
{
    if (!host)
        return FALSE;

    for (int i = 0; ALLOWED_HOSTS[i]; i++) {
        if (g_strcmp0(host, ALLOWED_HOSTS[i]) == 0)
            return TRUE;
    }

    /* Allow any *.forumline.net subdomain */
    return g_str_has_suffix(host, ".forumline.net");
}

static gboolean on_decide_policy(WebKitWebView *web_view,
                                 WebKitPolicyDecision *decision,
                                 WebKitPolicyDecisionType type,
                                 gpointer user_data)
{
    if (type != WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION)
        return FALSE;

    WebKitNavigationPolicyDecision *nav_decision = WEBKIT_NAVIGATION_POLICY_DECISION(decision);
    WebKitNavigationAction *action = webkit_navigation_policy_decision_get_navigation_action(nav_decision);
    WebKitURIRequest *request = webkit_navigation_action_get_request(action);
    const char *uri = webkit_uri_request_get_uri(request);

    GUri *parsed = g_uri_parse(uri, G_URI_FLAGS_NONE, NULL);
    if (!parsed)
        return FALSE;

    const char *scheme = g_uri_get_scheme(parsed);

    /* Allow internal schemes */
    if (g_strcmp0(scheme, "about") == 0 || g_strcmp0(scheme, "blob") == 0) {
        g_uri_unref(parsed);
        return FALSE;
    }

    const char *host = g_uri_get_host(parsed);

    if (is_host_allowed(host)) {
        g_uri_unref(parsed);
        return FALSE;
    }

    /* Open external links in the default browser */
    g_print("[Forumline] Blocked navigation to disallowed host: %s\n", host ? host : "(null)");
    webkit_policy_decision_ignore(decision);

    GError *error = NULL;
    gtk_show_uri(NULL, uri, GDK_CURRENT_TIME);
    (void)error;

    g_uri_unref(parsed);
    return TRUE;
}

static void on_script_message(WebKitUserContentManager *manager,
                              JSCValue *value,
                              gpointer user_data)
{
    if (!jsc_value_is_string(value))
        return;

    char *message = jsc_value_to_string(value);
    g_print("[Forumline] Bridge message: %s\n", message);

    /* Parse and handle messages here (auth-state, etc.)
     * For now, just log them. Full JSON parsing would use json-glib. */

    g_free(message);
}

static void on_load_changed(WebKitWebView *web_view,
                             WebKitLoadEvent load_event,
                             gpointer user_data)
{
    if (load_event == WEBKIT_LOAD_FINISHED) {
        g_print("[Forumline] Page loaded: %s\n", webkit_web_view_get_uri(web_view));
    }
}

static void activate(GtkApplication *app, gpointer user_data)
{
    GtkWidget *window = gtk_application_window_new(app);
    gtk_window_set_title(GTK_WINDOW(window), "Forumline");
    gtk_window_set_default_size(GTK_WINDOW(window), 1200, 800);

    /* Dark background matching other platform apps */
    GdkRGBA bg_color = { 15.0/255.0, 23.0/255.0, 42.0/255.0, 1.0 };

    /* Set up user content manager with bridge script */
    WebKitUserContentManager *content_manager = webkit_user_content_manager_new();

    WebKitUserScript *script = webkit_user_script_new(
        BRIDGE_JS,
        WEBKIT_USER_CONTENT_INJECT_ALL_FRAMES,
        WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START,
        NULL, NULL
    );
    webkit_user_content_manager_add_user_script(content_manager, script);
    webkit_user_script_unref(script);

    /* Register the message handler for web -> native */
    webkit_user_content_manager_register_script_message_handler(content_manager, "forumlineNative", NULL);
    g_signal_connect(content_manager, "script-message-received::forumlineNative",
                     G_CALLBACK(on_script_message), NULL);

    /* Create WebView */
    WebKitWebView *web_view = WEBKIT_WEB_VIEW(
        g_object_new(WEBKIT_TYPE_WEB_VIEW,
                     "user-content-manager", content_manager,
                     NULL)
    );

    webkit_web_view_set_background_color(web_view, &bg_color);

    /* Configure settings */
    WebKitSettings *settings = webkit_web_view_get_settings(web_view);
    webkit_settings_set_javascript_can_access_clipboard(settings, TRUE);
    webkit_settings_set_media_playback_requires_user_gesture(settings, FALSE);
    webkit_settings_set_enable_developer_extras(settings, TRUE);

    /* Connect signals */
    g_signal_connect(web_view, "decide-policy", G_CALLBACK(on_decide_policy), NULL);
    g_signal_connect(web_view, "load-changed", G_CALLBACK(on_load_changed), NULL);

    /* Load the app */
    webkit_web_view_load_uri(web_view, "https://app.forumline.net");

    gtk_window_set_child(GTK_WINDOW(window), GTK_WIDGET(web_view));
    gtk_window_present(GTK_WINDOW(window));
}

int main(int argc, char *argv[])
{
    GtkApplication *app = gtk_application_new("net.forumline.app", G_APPLICATION_DEFAULT_FLAGS);
    g_signal_connect(app, "activate", G_CALLBACK(activate), NULL);
    int status = g_application_run(G_APPLICATION(app), argc, argv);
    g_object_unref(app);
    return status;
}
