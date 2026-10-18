# Cursed GIT in a box.
#   docker run -it --rm ghcr.io/spiritpaper/cursed-git                    a playground: a throwaway project, Miranda in it
#   docker run -it --rm -v "$PWD:/work" ghcr.io/spiritpaper/cursed-git status   cg on your own project
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/spiritpaper/cursed-git/internal/cli.Version=${VERSION}" \
    -o /cg ./cmd/cg

FROM alpine:3.22
RUN apk add --no-cache git bash
COPY --from=build /cg /usr/local/bin/cg
COPY packaging/playground.sh /usr/local/bin/cg-playground
RUN adduser -D player \
 && git config --system --add safe.directory '*'
USER player
WORKDIR /home/player
ENV TERM=xterm-256color
LABEL org.opencontainers.image.source="https://github.com/spiritpaper/cursed-git" \
      org.opencontainers.image.description="Cursed GIT: like git, but she has feelings" \
      org.opencontainers.image.licenses="MIT"
ENTRYPOINT ["cg-playground"]
