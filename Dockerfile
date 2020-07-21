FROM golang:1.14

RUN curl -LO https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
RUN apt-get update && apt-get install -y ./google-chrome-stable_current_amd64.deb
RUN rm google-chrome-stable_current_amd64.deb 

WORKDIR /app

# Get MIDA dependencies
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy MIDA source code
COPY . .

RUN go build -o mida .

# Prometheus monitoring
EXPOSE 8001

CMD ["/app/mida", "client", "-c3","-s2","-l3"]
