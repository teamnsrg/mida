FROM ubuntu

RUN apt-get update && apt-get -y upgrade && apt-get -y install \
  python3 ca-certificates chromium-browser tcpdump

COPY setup.py /root

RUN python3 /root/setup.py 

COPY scripts/pcap-script.sh /root

RUN chmod +x /root/pcap-script.sh

ENTRYPOINT ["/root/pcap-script.sh"]
