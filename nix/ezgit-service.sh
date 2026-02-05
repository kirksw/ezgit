#!/usr/bin/env bash

set -e

PLIST_BUNDLE_ID="com.github.kirksw.ezgit"
PLIST_PATH="$HOME/Library/LaunchAgents/$PLIST_BUNDLE_ID.plist"
EZGIT_BIN="@out@/bin/ezgit"

log() {
    echo "[ezgit-service] $*"
}

check_launchctl_loaded() {
    launchctl list | grep -q "$PLIST_BUNDLE_ID"
}

install_service() {
    log "Installing ezgit cache refresh service..."

    if check_launchctl_loaded; then
        log "Service already loaded, reloading..."
        launchctl unload "$PLIST_PATH" 2>/dev/null || true
    fi

    log "Copying plist to $PLIST_PATH"
    install -m 644 "@out@/Library/LaunchAgents/$PLIST_BUNDLE_ID.plist" "$PLIST_PATH"

    log "Loading service..."
    launchctl load "$PLIST_PATH"

    log "Service installed successfully!"
    log "Cache will refresh every 24 hours automatically."
    log ""
    log "To view logs: tail -f /var/log/ezgit.log (may require sudo)"
    log "To manually refresh: $EZGIT_BIN cache refresh"
}

uninstall_service() {
    log "Uninstalling ezgit cache refresh service..."

    if check_launchctl_loaded; then
        log "Unloading service..."
        launchctl unload "$PLIST_PATH"
    fi

    if [ -f "$PLIST_PATH" ]; then
        log "Removing plist from $PLIST_PATH"
        rm -f "$PLIST_PATH"
    fi

    log "Service uninstalled successfully!"
}

status_service() {
    log "Checking ezgit cache refresh service status..."

    if check_launchctl_loaded; then
        log "✓ Service is loaded and running"
        log "  PID: $(launchctl list | grep "$PLIST_BUNDLE_ID" | awk '{print $1}')"
        log "  Last run: $(launchctl list | grep "$PLIST_BUNDLE_ID" | awk '{print $3}')"
    else
        log "✗ Service is not loaded"
        log ""
        log "To install: $EZGIT_BIN service install"
    fi
}

refresh_now() {
    log "Manually triggering cache refresh..."
    "$EZGIT_BIN" cache refresh --force
    log "Cache refresh complete!"
}

case "${1:-}" in
    install)
        install_service
        ;;
    uninstall)
        uninstall_service
        ;;
    status)
        status_service
        ;;
    refresh)
        refresh_now
        ;;
    *)
        echo "ezgit-service manager"
        echo ""
        echo "Usage: $EZGIT_BIN service {install|uninstall|status|refresh}"
        echo ""
        echo "Commands:"
        echo "  install    Install and start the cache refresh service"
        echo "  uninstall  Stop and remove the cache refresh service"
        echo "  status     Check service status"
        echo "  refresh    Manually trigger cache refresh now"
        echo ""
        echo "The service runs automatically every 24 hours."
        exit 1
        ;;
esac
