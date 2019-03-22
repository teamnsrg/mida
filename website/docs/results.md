
# Results Storage

## Storing Site Resources

Inside each per-visit directory, MIDA creates a file called `resources.json` containing
metadata about all resources loaded by that particular site. Almost all of this information
comes directly from from the DevTools Protocol. Below is the structure of this file. Note
that MIDA only calculates resource hashes and sizes if you have set the `all_files` option
to `True` in the task.

```
{
    requestId : {
        'requests': [
            {
                (DevTools data from Network.requestWillBeSent)
            }
        ],

        'response': {
            (DevTools Data from Network.responseReceived)
        },
    }
}
```

Again, the final three fields are not gathered unless the `all_files` option
was set in the task.  For further detail on the DevTools metadata provided,
see:

- [Network.requestWillBeSent](https://chromedevtools.github.io/devtools-protocol/tot/Network#event-requestWillBeSent)
- [Network.responseReceived](https://chromedevtools.github.io/devtools-protocol/tot/Network#event-responseReceived)


If the `all_files` option is set to `True`, a subdirectory called `files` will
be created, containing all files loaded by the website. Each file is named according
to the request ID it was assigned by Chromium. These are the same request IDs used as
keys in `resources.json`.
