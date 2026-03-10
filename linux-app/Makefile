PKG_CONFIG ?= pkg-config

GTK_CFLAGS := $(shell $(PKG_CONFIG) --cflags gtk4 webkitgtk-6.0)
GTK_LIBS := $(shell $(PKG_CONFIG) --libs gtk4 webkitgtk-6.0)

CFLAGS ?= -O2 -Wall -Wextra
PREFIX ?= /usr/local

forumline: forumline.c
	$(CC) $(CFLAGS) $(GTK_CFLAGS) -o $@ $< $(GTK_LIBS)

install: forumline
	install -Dm755 forumline $(DESTDIR)$(PREFIX)/bin/forumline
	install -Dm644 net.forumline.app.desktop $(DESTDIR)$(PREFIX)/share/applications/net.forumline.app.desktop

clean:
	rm -f forumline

.PHONY: install clean
