# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-FileCopyrightText: Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

FROM debian:bookworm-slim AS builder

# Prepare persistent data directory ownership for distroless nonroot user (UID/GID 65532)
RUN mkdir -p /data && chown 65532:65532 /data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /data /data

# Copy pre-built binaries from GoReleaser
COPY fru-tracker-server /usr/local/bin/fru-tracker-server

# Use distroless nonroot user (UID/GID 65532)
USER nonroot:nonroot

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/fru-tracker-server"]
CMD ["serve"]
