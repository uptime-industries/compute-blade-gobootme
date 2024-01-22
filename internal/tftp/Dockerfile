FROM ubuntu:22.04 AS builder

ENV IPXE_SHA=de8a0821c7bc737e724fa3dfb6d89dc36f591d7a

RUN apt-get update && apt-get install -y \
        build-essential \
        gcc-aarch64-linux-gnu \
        gcc-x86-64-linux-gnu \
        curl \
        git

WORKDIR /ipxe


# Download iPXE source code
RUN curl -sfL -o ipxe.tar.gz https://github.com/ipxe/ipxe/archive/$IPXE_SHA.tar.gz && \
    tar xf ipxe.tar.gz && \
    rm ipxe.tar.gz && \
    mv ipxe-$IPXE_SHA/* .

COPY https.patch /ipxe/https.patch
RUN patch -p1 < https.patch

COPY boot.ipxe /ipxe/boot.ipxe


FROM builder AS build-arm64
# Build iPXE for arm64
RUN make -j $(nproc --all) -C src \
        CROSS=aarch64-linux-gnu- \
        bin-arm64-efi/ipxe.efi \
        EMBED=../boot.ipxe \
        VERSION=gobootme-${IPXE_SHA}


FROM builder AS build-x86_64
# Build iPXE for x86_64
RUN make -j $(nproc --all) -C src \
        CROSS=x86_64-linux-gnu- \
        bin-x86_64-efi/ipxe.efi \
        EMBED=../boot.ipxe \
        VERSION=gobootme-${IPXE_SHA}


FROM scratch AS release
COPY --from=build-arm64 /ipxe/src/bin-arm64-efi/ipxe.efi /arm64.efi
COPY --from=build-x86_64 /ipxe/src/bin-x86_64-efi/ipxe.efi /amd64.ef

