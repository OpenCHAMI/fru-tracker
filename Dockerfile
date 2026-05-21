# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-FileCopyrightText: Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Pairs with .goreleaser.yaml's CGO_ENABLED=0 build: the resulting
# fru-tracker-server binary is statically linked and only needs a
# CA bundle for outbound TLS. distroless/static-debian12 ships
# /etc/ssl/certs from ca-certificates and includes the nonroot
# user (UID/GID 65532) — no apt-get step, no useradd, no chown,
# nothing to break under multi-arch QEMU emulation. Image is ~10 MB.
#
# Two-stage build only to pre-stage a writable /data directory
# owned by the distroless nonroot user; the final image has no
# shell so we can't chown there.
FROM debian:bookworm-slim AS data-stage
RUN mkdir -p /data && chown 65532:65532 /data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=data-stage /data /data

# Copy the pre-built binary GoReleaser dropped into the build context.
COPY fru-tracker-server /usr/local/bin/fru-tracker-server

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/fru-tracker-server"]
CMD ["serve"]
