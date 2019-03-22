# **MIDA: A Tool for Measuring the Web**

MIDA is a highly configurable web crawler built on top of Chromium
and the Chrome DevTools Protocol. It can gather more information
about websites than any other packaged crawler we are aware of, and
it can be run locally from a single machine or as the backbone of
a distributed crawling infrastructure.

---

Getting started with MIDA is easy! First, install:
```
wget files.mida.sprai.org/mida_setup.py
sudo python3 mida_setup.py
```

Now you're ready to gather some data:
```
mida go www.illinois.edu
```
You can find the results of your crawl in the `results/` directory.

---
This project is under active development and should not be considered
stable. Email questions to [Paul Murley](mailto:pmurley2@illinois.edu).
