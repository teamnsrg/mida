# MIDA: A Tool for Measuring the Web

[![Build Status](https://travis-ci.com/teamnsrg/mida.svg?branch=master)](https://travis-ci.com/teamnsrg/mida)
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

## Docker

The Dockerfile is intended for running the measurement tool along with a packet capture. It 
downloads a single site. To run it:

```bash
$ docker run --rm -v $HOME/data:/data mida site.com
```

---

You can view more MIDA documentation at [mida.sprai.org](https://mida.sprai.org). You can also
keep up with development progress via [our Trello board](https://trello.com/b/KSpQS5jk/mida).
