# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-FileCopyrightText: Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    bash

# Create non-root user
RUN addgroup -g 1000 fru && \
    adduser -D -u 1000 -G fru fru

WORKDIR /home/fru

# Copy pre-built binaries from GoReleaser
COPY fru-tracker-server /usr/local/bin/fru-tracker-server

# Set ownership
RUN chown -R fru:fru /home/fru

# Switch to non-root user
USER fru

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/fru-tracker-server"]
CMD ["serve"]
