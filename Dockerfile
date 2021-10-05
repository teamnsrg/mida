FROM ubuntu:latest

ARG CACHEBUSTER=1

RUN apt-get update && apt-get -y upgrade
RUN apt-get install -y curl

# Install our version of llvm-profdata, llvm-cov, chromium binary for processing Clang coverage data
RUN cd /usr/bin && { curl -O https://files.mida.sprai.org/coverage/llvm-profdata ; chmod +x llvm-profdata ; cd -; }
RUN cd /usr/bin && { curl -O https://files.mida.sprai.org/coverage/llvm-cov-custom ; chmod +x llvm-cov-custom ; cd -; }
RUN cd /usr/bin && { curl -O https://files.mida.sprai.org/browser/chrome_91_cov_unstripped ; cd -; }

# Install latest version of chrome
# RUN apt-get update
# RUN apt-get install -yq software-properties-common
# RUN add-apt-repository ppa:ubuntu-toolchain-r/test
# RUN apt-get update
# RUN apt-get -y upgrade

RUN curl -LO https://files.mida.sprai.org/browser/chromium_91_cov.deb
RUN ln -fs /usr/share/zoneinfo/America/Chicago /etc/localtime
RUN apt-get install -y ./chromium_91_cov.deb
RUN rm -f chromium_91_cov.deb
# RUN apt-get autoclean
# RUN apt-get update
# RUN apt-get upgrade
# RUN apt-get -uy dist-upgrade
# RUN dpkg --configure -a
# RUN apt-get -f install



RUN apt-get install -yq gconf-service libasound2 libatk1.0-0 libc6 libcairo2 libcups2 libdbus-1-3 libexpat1 libfontconfig1 libgcc1 libgconf-2-4 libgdk-pixbuf2.0-0 libglib2.0-0 libgtk-3-0 libnspr4 libpango-1.0-0 libpangocairo-1.0-0 libstdc++6 libx11-6 libx11-xcb1 libxcb1 libxcomposite1 libxcursor1 libxdamage1 libxext6 libxfixes3 libxi6 libxrandr2 libxrender1 libxss1 libxtst6 ca-certificates fonts-liberation libappindicator1 libnss3 lsb-release xdg-utils wget x11vnc x11-xkb-utils xfonts-100dpi xfonts-75dpi xfonts-scalable xfonts-cyrillic x11-apps xvfb


RUN apt-get install dumb-init

RUN wget https://golang.org/dl/go1.17.1.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.17.1.linux-amd64.tar.gz && rm go1.17.1.linux-amd64.tar.gz
ENV PATH="${PATH}:/usr/local/go/bin"

WORKDIR /mida

COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy MIDA source code into container
COPY . .

RUN go build
RUN mv mida /usr/bin/mida
RUN rm -rf *

# Install the latest version of MIDA
# RUN wget files.mida.sprai.org/setup.py
# RUN python3 setup.py
# RUN rm setup.py

# Required so there is an init() process to kill zombies
ENTRYPOINT ["/usr/bin/dumb-init", "--"]

CMD ["mida", "client", "-l3", "--xvfb"]
