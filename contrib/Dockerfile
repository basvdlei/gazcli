FROM golang:1.19-alpine AS builder
ENV GO_ENABLED=0
RUN go install github.com/basvdlei/gazcli/cmd/gazcli@latest

FROM mcr.microsoft.com/azure-cli:latest
ADD https://raw.githubusercontent.com/urfave/cli/main/autocomplete/bash_autocomplete /etc/bash_completion.d/gazcli
COPY --from=builder /go/bin/gazcli /usr/local/bin/gazcli
