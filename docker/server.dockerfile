ARG PLATFORM=amd64
FROM ${PLATFORM}/ubuntu:24.04 AS builder

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
  curl \
  unzip \
  wget \
 && rm -rf /var/lib/apt/lists/*

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
  git \
  cmake \
  make \
  build-essential \
 && rm -rf /var/lib/apt/lists/*

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
  libgstreamer1.0-dev \
  libgstreamer-plugins-base1.0-dev \
  libgstreamer-plugins-bad1.0-dev \
 && rm -rf /var/lib/apt/lists/*

ENV GO_VERSION=1.23.4
RUN wget -P /tmp "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz"

RUN tar -C /usr/local -xzf "/tmp/go${GO_VERSION}.linux-amd64.tar.gz"
RUN rm "/tmp/go${GO_VERSION}.linux-amd64.tar.gz"

ENV GOPATH=/go
ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

RUN sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

COPY .. /opt/app/
#COPY Taskfile.yml /opt/app/Taskfile.yml

WORKDIR /opt/app
RUN task build-server

FROM ${PLATFORM}/ubuntu:24.04

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y \
  gstreamer1.0 \
  gstreamer1.0-vaapi \
  gstreamer1.0-plugins-good \
  gstreamer1.0-plugins-bad \
  gstreamer1.0-tools \
  alsa-utils \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /opt/app/livekit-video-server /opt/app/livekit-video-server

WORKDIR /opt/app

CMD ["/opt/app/livekit-video-server", "serve"]