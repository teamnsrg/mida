FROM golang:1.14

# Install latest version of chrome
RUN curl -LO https://files.mida.sprai.org/browser/chromium_84_cov.deb
RUN apt-get update && apt-get install -y ./chromium_84_cov.deb
RUN rm chromium_84_cov.deb 
RUN apt-get install -yq gconf-service libasound2 libatk1.0-0 libc6 libcairo2 libcups2 libdbus-1-3 libexpat1 libfontconfig1 libgcc1 libgconf-2-4 libgdk-pixbuf2.0-0 libglib2.0-0 libgtk-3-0 libnspr4 libpango-1.0-0 libpangocairo-1.0-0 libstdc++6 libx11-6 libx11-xcb1 libxcb1 libxcomposite1 libxcursor1 libxdamage1 libxext6 libxfixes3 libxi6 libxrandr2 libxrender1 libxss1 libxtst6 ca-certificates fonts-liberation libappindicator1 libnss3 lsb-release xdg-utils wget x11vnc x11-xkb-utils xfonts-100dpi xfonts-75dpi xfonts-scalable xfonts-cyrillic x11-apps xvfb

# Install our version of llvm-cov for processing Clang coverage data
RUN cd /usr/bin && { curl -O https://files.mida.sprai.org/coverage/llvm-profdata ; chmod +x llvm-profdata ; cd -; }

RUN apt-get install dumb-init

WORKDIR /mida

COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy MIDA source code into container
COPY . .

RUN go install
RUN rm -rf *

# Install the latest version of MIDA
# RUN wget files.mida.sprai.org/setup.py
# RUN python3 setup.py
# RUN rm setup.py

# Required so there is an init() process to kill zombies
ENTRYPOINT ["/usr/bin/dumb-init", "--"]

CMD ["mida", "client", "-l3", "--xvfb"]
