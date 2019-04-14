#!/bin/bash

tcpdump -i eth0 -w /trace.pcap & 

export SSLKEYLOGFILE=/ssl.log

/usr/local/bin/mida go --add-browser-flags=headless,disable-gpu $@

kill -1 %%

sleep 3

mv -v /trace.pcap /ssl.log /results/*/*/

cp -av /results/* /data


