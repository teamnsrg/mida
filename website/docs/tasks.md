# **Defining Tasks for MIDA**

## Task Structure

---

Tasks are the unit of work MIDA recognizes. A task consists of exactly
one visit to exactly one URL. Tasks are represented as JSON files when stored. Below is
the simplest possible structure for a valid task, containing only a single URL
to crawl:

```json
{
    "url": "illinois.edu"
}
```

In the same above, defaults will be used for all parameters other than the URL.
Tasks can specify many other options. Below is a more complex, customized task
where some parameters are set explicitly:


```json
{
  "url": "http://cnn.com",
  "browser": {
    "browser_binary": "/home/mida/browser/chrome",
    "user_data_directory": "",
    "add_browser_flags": ["headless", "disable-gpu"],
    "remove_browser_flags": [],
    "set_browser_flags": [],
    "extensions": ["/home/mida/extensions/uBlock/"]
  },
  "completion": {
    "completion_condition": "CompleteOnTimeoutOnly",
    "timeout": 30,
    "time_after_load": 0
  },
  "data": {
    "all_files": true,
    "all_scripts": false,
    "js_trace": true,
    "save_raw_trace": false,
    "resource_metadata": true,
    "script_metadata": true,
    "resource_tree": false,
	"websocket": false
  },
  "output": {
    "path": "results/",
    "group_id": "default"
  },
  "max_attempts": 2
}
```
---

## Building Tasks and Task Sets

We expect most crawls using MIDA to cover a list of URLs while keeping most or
all of the other task parameters consistent. To this end, MIDA can create an interpret
compressed task sets. Compressed task sets are structured exactly like tasks, except
that the `url` field is an array of strings rather than a single string.

Given a file containing a list of URLs (one per line), you can construct a compressed
task set in this way:

```
$ mida build -f urls.lst -o compressedTaskSet.json
```

You can add arguments to this command to tune specific task parameters. The following
command creates a compressed task set which gathers all files and remains on each page
for 35 seconds:
```
$ mida build -f urls.lst -t 35 --all-files -o compressedTaskSet.json
```
You can always manually inspect/edit the JSON task files after they are created to ensure
they meet your needs.

---

## Task Parameters

#### `url` (**REQUIRED**)
- type: `string` or `[]string`
- description: The URL that will be initially navigated to for this task. Note that MIDA
  will follow redirections just as a typical browser would

#### `max_attempts`
- type: `int`
- default: 2
- description: MIDA automatically retries tasks when they fail. Failures can
  happen for many reasons (bad URL, browser crashed, results storage failed,
  etc). This option sets how many times MIDA will retry a failed task before it
  gives up.

### Browser Parameters

#### `add_browser_flags`
- type: `string` or `[]string`
- default: **None**
- description: Add command line arguments to the browser, in addition to the
  set of [default flags](/browser#Browser_Flags)

#### `browser_binary`
- type: `string`
- default: First choice Chromium, second choice Chrome. Fails if neither is present.
- description: The browser binary to use for crawling. Needs to be either
  Chrome or Chromium since DevTools is the backbone of how the crawler
  operates.

#### `extensions`
- type: `string` or `[]string`
- default: **None**
- description: Path(s) to extension(s) to use in this crawl. Note that Chrome extensions
  cannot be used in headless or incognito mode, so setting extensions will override those
  options. We provide some sample extensions [here](http://files.mida.sprai.org/extensions).

#### `remove_browser_flags`
- type: `string` or `[]string`
- default: **None**
- description: Remove flags from the list of default flags used for crawls. If
  the argument or arguments do not match a default flag character for
  character, they will be silently ignored.

#### `set_browser_flags`
- type: `[]string`
- default: **None**
- description: Overrides default flags passed to the browser with this list,
  exactly as given. This option also overrides
  [`add_browser_flags`](#add_browser_flags) and
  [`remove_browser_flags`](#remove_browser_flags). MIDA assumes certain flags
  will be set, so this option should be used with caution.


### Task Completion Parameters

#### `completion_condition`
- type: `string`
- default: `CompleteOnTimeoutOnly`
- description: Condition on which MIDA will complete a visit to a website. Currently,
  MIDA offers the following three options:
    - `CompleteOnTimeoutOnly`: Leave page when specified timeout is reached, no matter what
    - `CompleteOnLoadEvent`: Leave page when the load event fires, or when the timeout is
    reached (whichever comes first).
    - `CompleteOnTimeoutAfterLoad`: Leave page when 1) The load event occurs and then we wait
    for `time_after_load` seconds, or 2) `timeout` is reached, whichever comes sooner.

#### `timeout`
- type: `int`
- default: `60`
- description: Time (in seconds) to remain on each page. The crawl will end and
  results will be stored after this interval, no matter whether the page has
  completed loading or not.

#### `time_after_load`
- type: `int`
- default: *None*
- description: Time (in seconds) to remain on the page after it fires its
  first load event. This parameter is only used if `completion_condition` is
  `CompleteOnTimeoutAfterLoad`. If `timeout` is reached before `time_after_load`,
  then `time_after_load` is disregarded.

### Data Gathering Parameters

#### `all_files`
- type: `bool`
- default: `false`
- description: Whether to store all files/resources downloaded while visiting
  the site. 

#### `all_scripts`
- type: `bool`
- default: `false`
- description: Whether to store all unique scripts parsed by the browser while
visiting the target site. 

#### `js_trace`
- type: `bool`
- default: `false`
- description: Whether to gather a trace of JavaScript browser API calls made
  by scripts on the target site, including arguments and return values. This option
  requires the use of an instrumented version of Chromium, and creates a JSON file
  containing the full trace. **Note: These traces are often several
  megabytes all by themselves.**

#### `resource_metadata`
- type: `bool`
- default: `true`
- description: Whether to gather metadata on the resources requested by the browser
  during the crawl. This metadata amounts to the data provided by the DevTools Network
  domain events
  [`network.requestWillBeSent`](https://chromedevtools.github.io/devtools-protocol/tot/Network#event-requestWillBeSent)
  and
  [`network.responseReceived`](https://chromedevtools.github.io/devtools-protocol/tot/Network#event-responseReceived)

#### `resource_tree`
- type: `bool`
- default: `false`
- description: MIDA contains logic to construct a best-effort dependency tree
  of resources, starting with frames and then using URLs. This option causes
  this tree to be built and stored.

#### `save_raw_trace`
- type: `bool`
- default: `true`
- description: Whether to save the raw trace (browser log). Note that browsers
  instrumented to record JavaScript calls often produce browser logs which are
  megabytes or tens of megabytes for each site visited.  Without an
  instrumented browser, there is no reason to use this option.

#### `script_metadata`
- type: `bool`
- default: `true`
- description: Whether to gather metadata on the scripts (JavaScript) parsed by the browser
  during the crawl. This metadata amounts to the data provided by the DevTools Debugger
  domain event
  [`debugger.scriptParsed`](https://chromedevtools.github.io/devtools-protocol/tot/Debugger#event-scriptParsed)

#### `websocket`
- type: `bool`
- default: `false`
- description: Whether to gather data on websocket connections established during the crawl and data passing through them.

### Output Parameters

#### `path`
- type: `string`
- default: `results/`
- description: Path under which results will be stored. MIDA creates its own
  directory structure within this path where it stores results from individual
  tasks. `path` may also specify a remote location via SSH (e.g.
  `ssh://my.server.com/mida_results/`).  Currently, this requires key
  authentication using the default system SSH key (`~/.ssh/id_rsa`).
  While an output path parameter not being present will be automatically set
  to `results/`, an emtpy output path will result in the standard results directory
  not being stored.

#### `group_id`
- type: `string`
- default: `default`
- description: `group_id` is meant to help group experiments run with MIDA in various backends.
  Set this parameter to a descriptive name for the particular experiment you are running to allow
  you to more easily group relevant tasks together for analysis later.

