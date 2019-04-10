FROM ubuntu

RUN apt-get update && apt-get -y upgrade && apt-get -y install \
  python3 ca-certificates chromium-browser xvfb

COPY setup.py /root

RUN python3 /root/setup.py 
