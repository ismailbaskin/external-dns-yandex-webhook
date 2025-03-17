FROM gcr.io/distroless/cc-debian12:nonroot

COPY external-dns-yandex-webhook /external-dns-yandex-webhook
ENTRYPOINT ["/external-dns-yandex-webhook"]
