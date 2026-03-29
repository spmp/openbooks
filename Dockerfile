FROM node:16 as web
WORKDIR /web
COPY . .
WORKDIR /web/server/app/
RUN npm install
RUN npm run build

FROM golang as build
WORKDIR /go/src/
COPY . .
COPY --from=web /web/ .

ENV CGO_ENABLED=0
RUN go get -d -v ./...
RUN go install -v ./...
WORKDIR /go/src/cmd/openbooks/
RUN go build

FROM python:3.14.2-slim as app
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends bash ca-certificates git && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /books && chmod 0777 /books
COPY --from=build /go/src/cmd/openbooks/openbooks .

# Install the ebooks script
RUN pip install "git+https://github.com/spmp/ebook-resolve-move"

EXPOSE 5228
ENV BASE_PATH=/

ENTRYPOINT ["./openbooks", "server"]
