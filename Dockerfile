FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    software-properties-common \
    wget \
    git \
    build-essential \
    cmake \
    clang-18 \
    libc++-18-dev \
    libc++abi-18-dev \
    zlib1g-dev \
    libssl-dev \
    gperf \
    php-cli \
    golang-go \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Need to install Go
# RUN wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz && \
#     rm -rf /usr/local/go && tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz && \
#     echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# Install Tdlib
# WORKDIR /tmp
# RUN git clone https://github.com/tdlib/td.git
# WORKDIR /tmp/td
# RUN rm -rf build && mkdir build && cd build && \
#     CXXFLAGS="-stdlib=libc++" \
#     CC=/usr/bin/clang-18 \
#     CXX=/usr/bin/clang++-18 \
#     cmake -DCMAKE_BUILD_TYPE=Release \
#           -DCMAKE_INSTALL_PREFIX:PATH=/usr/local \
#           .. && \
#     cmake --build . --target install -j$(nproc)

WORKDIR /app
COPY . .

ENV CGO_ENABLED=1 \
    CGO_CFLAGS="-I/usr/local/include" \
    CGO_LDFLAGS="-L/usr/local/lib -ltdjson -Wl,-rpath,/usr/local/lib" \
    GOOS=linux \
    GOARCH=amd64

RUN go build -o main ./cmd/telegram-client

CMD ["./main"]