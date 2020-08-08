# MIDA: A Tool for Measuring the Web

![Build](https://github.com/teamnsrg/mida/workflows/Go/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/teamnsrg/mida)](https://goreportcard.com/report/github.com/teamnsrg/mida)

MIDA is meant to be a general tool for web measurement projects. It is built in Go 
on top of Chrome/Chromium and the DevTools protocol, giving it a realistic vantage point
to study the web and fine-grained access to information provided by Chrome Developer Tools.

---

## Getting Started

Getting started with MIDA is easy! First, install:

```bash
$ wget files.mida.sprai.org/setup.py
$ sudo python3 setup.py 
```

Now we are ready to visit a site and collect some data:
```bash
$ mida go www.illinois.edu
```

You can find the results of your crawl in the `results/` directory.

## Easy At-Scale Crawling

One major benefit of MIDA is in being able to run large scale, highly configurable crawls
without needing to write your own crawler code. Here's an example of a single MIDA command which
will crawl the Alexa Top 100K and gather a few specific types of data:

```bash
$ mida go -f https://files.mida.sprai.org/toplists/alexa.lst -n100000 -c8 --all-resources --screenshot --dom
```

Breaking this down by argument:

`-f https://files.mida.sprai.org/toplists/alexa.lst`: This is a list of the Alexa Top Websites.
You can read from a local file or go get one hosted on the web somewhere

`-n100000`: Read the top 100,000 entries from the list

`-c8`: Run with 8 parallel crawlers (browser instances)

`--all-resources`: Gather all of the actual files/resources required to render the web page.
Beware, this takes a lot of space!

`--screenshot`: Capture a screenshot after/if the load event for each website fires.

`--dom`: Capture a JSON representation of the DOM for each website visited.