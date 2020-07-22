# MIDA: A Tool for Measuring the Web

![Build](https://github.com/teamnsrg/mida/workflows/Go/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/teamnsrg/mida)](https://goreportcard.com/report/github.com/teamnsrg/mida)

MIDA is meant to be a general tool for web measurement projects. It is built in Go 
on top of Chrome/Chromium and the DevTools protocol, giving it a realistic vantage point
to study the web and fine-grained access to information provided by Chrome Developer Tools.

---

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
