FROM golang:1.19 AS builder

# Copy MIDA source code into container
WORKDIR /mida

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build

FROM ubuntu:focal

RUN apt-get update && apt-get -y upgrade
RUN apt-get install -y wget

RUN ln -fs /usr/share/zoneinfo/America/Chicago /etc/localtime

RUN wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
RUN apt-get install -y ./google-chrome-stable_current_amd64.deb
RUN rm google-chrome-stable_current_amd64.deb

RUN apt-get autoclean
RUN apt-get update
RUN apt-get upgrade
RUN apt-get -uy dist-upgrade
RUN dpkg --configure -a
RUN apt-get -f install
RUN apt-get install -y dumb-init xvfb

COPY --from=builder /mida/mida /usr/bin/mida

# Required so there is an init() process to kill zombies
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["mida", "client", "-l3", "--xvfb"]

