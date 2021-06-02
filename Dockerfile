# Build Elm Compiler
FROM alpine:3.11 AS elm-compiler-env
RUN apk add --no-cache git ghc cabal wget musl-dev zlib-dev zlib-static ncurses-dev ncurses-static
WORKDIR /elm
RUN git clone https://github.com/MichaelCombs28/elm-kernel-compiler.git
RUN cd elm-kernel-compiler && rm -rf worker docs hints installers
RUN cd elm-kernel-compiler && cabal new-update
RUN cd elm-kernel-compiler && cabal new-build --ghc-option=-optl=-static --ghc-option=-split-sections
RUN cp ./elm-kernel-compiler/dist-newstyle/build/x86_64-linux/ghc-*/elm-*/x/elm/build/elm/elm /usr/local/bin/elm
RUN strip -s /usr/local/bin/elm

# Build Go Binary
FROM golang:alpine AS build-env
RUN apk --no-cache add build-base git
COPY elmproxy /src/elmproxy/
COPY main.go go.mod go.sum /src/
RUN cd /src && go build

FROM alpine
EXPOSE 8080
EXPOSE 8081
WORKDIR /app
VOLUME /app/data
COPY --from=build-env /src/elm-package-proxy /app/
COPY --from=elm-compiler-env /usr/local/bin/elm /usr/local/bin/elm-sh
COPY config.yml ca.crt ca.key ./
RUN echo "HTTPS_PROXY=http://localhost:8080 elm-sh" > /usr/local/bin/elm
RUN chmod a+x /usr/local/bin/elm
CMD ./elm-package-proxy
