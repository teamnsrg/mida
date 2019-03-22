# Managing the Browser

## Browser Flags

To facilitate intuitive automation settings, MIDA has a set of flags which it passes
to the browser process by default:

```
--disable-background-networking
--disable-background-timer-throttling
--disable-client-side-phishing-detection
--disable-extensions
--disable-features=IsolateOrigins,site-per-process
--disable-ipc-flooding-protection
--disable-popup-blocking
--disable-prompt-on-repost
--disable-renderer-backgrounding
--disable-sync
--disk-cache-size=0
--incognito
--new-window
--no-default-browser-check
--no-first-run
--safebrowsing-disable-auto-update
```

Users can add to this list using the
[`add_browser_flags`](/tasks.md#add_browser_flags) options or subtract from it
using the [`remove_browser_flags`](/tasks.md#remove_browser_flags) option.
Alternatively, users can completely override this list using the
[`set_browser_flags`](/tasks.md#set_browser_flags) option. This should be done
with extreme caution, as changing some flags will interfere with MIDA's data gathering
capabilities. For example, disabling site isolation is currently required in order
for DevTools to gather information from frames other than the primary frame.

For an in-depth guide to Chromium browser flags, see
[here](https://peter.sh/experiments/chromium-command-line-switches/).
