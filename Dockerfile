# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-FileCopyrightText: Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

# Pairs with .goreleaser.yaml's CGO_ENABLED=1 build for sqlite3
# support. A CGO binary expects glibc at runtime, so we use the
# standard distroless Debian base rather than the static variant.
#
# Two-stage build only to pre-stage a writable /data directory
# owned by the distroless nonroot user; the final image has no
# shell so we can't chown there.
FROM debian:bookworm-slim AS data-stage
RUN mkdir -p /data && chown 65532:65532 /data

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=data-stage /data /data

# Copy the pre-built binary GoReleaser dropped into the build context.
COPY fru-tracker-server /usr/local/bin/fru-tracker-server

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/fru-tracker-server"]
CMD ["serve"]
