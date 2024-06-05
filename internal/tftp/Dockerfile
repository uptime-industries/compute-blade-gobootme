FROM ubuntu:22.04 AS ipxe-arm64

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

# Build iPXE for arm64
RUN make -j $(nproc --all) -C src \
        CROSS=aarch64-linux-gnu- \
        bin-arm64-efi/ipxe.efi \
        EMBED=../boot.ipxe \
        VERSION=gobootme-${IPXE_SHA}



FROM ubuntu:22.04 AS rpi-uefi

ENV CM4_UEFI_VERSION=v0.1.1

RUN apt-get update && apt-get install -y curl zip
WORKDIR /rpi-uefi

RUN curl -LO "https://github.com/Uptime-Lab/computeblade-cm4-uefi/releases/download/${CM4_UEFI_VERSION}/dist.zip" && \
    unzip dist.zip && \
    rm dist.zip


FROM scratch AS release
COPY --from=ipxe-arm64 /ipxe/src/bin-arm64-efi/ipxe.efi /snp.efi
COPY --from=rpi-uefi /rpi-uefi/dist/ /
