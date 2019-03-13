#!/bin/sh

cd ~/go/src/github.com/pmurley/mida
go build
MIDA_HASH=$(sha256sum mida)
SETUP_SCRIPT_HASH=$(sha256sum scripts/mida_setup.py)
echo $MIDA_HASH > sha256sums.txt
echo $SETUP_SCRIPT_HASH >> sha256sums.txt
scp mida files.mida.sprai.org:.mida.tmp
scp scripts/mida_setup.py files.mida.sprai.org:.mida_setup.tmp
scp sha256sums.txt files.mida.sprai.org:sha256sums.txt
ssh -t files.mida.sprai.org "
  sudo mv .mida.tmp /var/www/files.mida.sprai.org/mida;
  sudo mv .mida_setup.tmp /var/www/files.mida.sprai.org/mida_setup.py;
  sudo mv sha256sums.txt /var/www/files.mida.sprai.org/sha256sums.txt;
"
rm sha256sums.txt
