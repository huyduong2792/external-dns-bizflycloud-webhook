FROM gcr.io/distroless/static-debian11:nonroot

USER 20000:20000
ADD --chmod=555 external-dns-bfc-webhook /opt/external-dns-bfc-webhook/app

ENTRYPOINT ["/opt/external-dns-bfc-webhook/app"]