# Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
# SPDX-FileCopyrightText: Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
#
# SPDX-License-Identifier: MIT

FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    git \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 fru && \
    useradd -r -u 1000 -g fru fru

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
