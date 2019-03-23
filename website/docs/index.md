# **MIDA: A Tool for Measuring the Web**

MIDA is a configurable web crawler built on top of Chromium
and the Chrome DevTools Protocol. It gathers a broad range of information
about the websites it visits, and it can be run locally from a single machine or
used as the backbone of a distributed crawling infrastructure.

MIDA runs on OS X (developed on High Sierra) and Ubuntu (developed on 18.04).
It requires an installation of a browser drivable using the Chrome DevTools Protocol.

---
Getting started with MIDA is easy!

1. Install:
```
$ wget files.mida.sprai.org/setup.py
$ sudo python3 setup.py
```

2. Gather data:
```
$ mida go www.illinois.edu
```
You can find the results of your crawl in the `results/` directory.

---
This project is under active development and should not be considered
stable.  Please feel free to email [Paul Murley](mailto:pmurley2@illinois.edu)
with questions or bug reports.
