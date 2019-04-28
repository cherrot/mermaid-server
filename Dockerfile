FROM golang AS builder
RUN apt-get update \
        && apt-get install -y git --no-install-recommends \
        && rm -rf /var/lib/apt/lists/*
WORKDIR /mermaid
COPY . .
RUN go build

FROM buildkite/puppeteer
COPY --from=builder /mermaid/mermaid-server /usr/bin/
RUN npm install mermaid.cli
ENV PATH=$PATH:/node_modules/.bin
RUN echo '{"args": ["--no-sandbox"]}' > /puppeteer.json
WORKDIR /data
CMD mermaid-server --exec='mmdc -p /puppeteer.json' --port=8100 --httpRoot=/mermaid/ --fileRoot=./
