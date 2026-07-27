# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ARG TARGETOS
ARG TARGETARCH
COPY bin/reloader-${TARGETOS}-${TARGETARCH} /bin/reloader
# Run as UID for nobody
USER 65534

ENTRYPOINT ["/bin/reloader"]
