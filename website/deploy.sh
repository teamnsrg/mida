#!/bin/sh

# Deploys the site to the webserver
# This should work as long as you have sudo access on webserver.sprai.org

rm -rf site
mkdocs build

scp -r site webserver.sprai.org:mida.sprai.org
ssh -t webserver.sprai.org "sudo rm -rf /var/www/mida.sprai.org && sudo cp -Tr mida.sprai.org /var/www/mida.sprai.org && rm -rf mida.sprai.org"
