#!/bin/bash

cd ~/go/src/github.com/teamnsrg/mida
GOOS=linux GOARCH=amd64 go build -o mida_linux_amd64
GOOS=darwin GOARCH=amd64 go build -o mida_darwin_amd64
if [[ "$OSTYPE" == "linux-gnu" ]]; then
    LINUX_HASH=$(sha256sum mida_linux_amd64)
    DARWIN_HASH=$(sha256sum mida_darwin_amd64)
    SETUP_SCRIPT_HASH=$(sha256sum scripts/setup.py)
elif [[ "$OSTYPE" == "darwin"* ]]; then
    LINUX_HASH=$(gsha256sum mida_linux_amd64)
    DARWIN_HASH=$(gsha256sum mida_darwin_amd64)
    SETUP_SCRIPT_HASH=$(gsha256sum scripts/setup.py)
fi

> sha256sums.txt
echo ${LINUX_HASH} >> sha256sums.txt
echo ${DARWIN_HASH} >> sha256sums.txt
echo ${SETUP_SCRIPT_HASH} >> sha256sums.txt

scp mida_linux_amd64 files.mida.sprai.org:.mida.linux.amd64.tmp
scp mida_darwin_amd64 files.mida.sprai.org:.mida.darwin.amd64.tmp
scp scripts/setup.py files.mida.sprai.org:.setup.tmp
scp sha256sums.txt files.mida.sprai.org:sha256sums.txt
ssh -t files.mida.sprai.org "
  sudo mv .mida.darwin.amd64.tmp /var/www/files.mida.sprai.org/mida_darwin_amd64;
  sudo mv .mida.linux.amd64.tmp /var/www/files.mida.sprai.org/mida_linux_amd64;
  sudo mv .setup.tmp /var/www/files.mida.sprai.org/setup.py;
  sudo mv sha256sums.txt /var/www/files.mida.sprai.org/sha256sums.txt;
"
rm mida_linux_amd64
rm mida_darwin_amd64
rm sha256sums.txt
