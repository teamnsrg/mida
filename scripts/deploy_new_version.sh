#!/bin/sh

cd ~/go/src/github.com/pmurley/mida
go build
scp mida files.mida.sprai.org:.mida.tmp
scp scripts/mida_setup.py files.mida.sprai.org:.mida_setup.tmp
ssh -t files.mida.sprai.org "
  sudo cp .mida.tmp /var/www/html/mida;
  sudo cp .mida_setup.tmp /var/www/html/mida_setup.py;
  rm .mida.tmp .mida_setup.tmp
"
