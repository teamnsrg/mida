#!/bin/bash

tcpdump -i eth0 -w /trace.pcap & 

/usr/local/bin/mida go --add-browser-flags=headless,disable-gpu $@

kill %%

mv -v /trace.pcap /results/*/*/

cp -av /results/* /data


