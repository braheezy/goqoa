FROM fedora:latest as builder

RUN dnf install -y gcc make curl-devel alsa-lib-devel git golang && \
    dnf clean all

WORKDIR /app

RUN git clone https://github.com/phoboslab/qoa.git --depth 1 && \
    cd qoa && \
    curl -L https://github.com/mackron/dr_libs/raw/master/dr_mp3.h -o dr_mp3.h && \
    curl -L https://github.com/mackron/dr_libs/raw/master/dr_flac.h -o dr_flac.h && \
    curl -L https://github.com/floooh/sokol/raw/master/sokol_audio.h -o sokol_audio.h && \
    make conv

COPY go.mod .
RUN go mod download

COPY . .
RUN go build -o goqoa .

FROM fedora:latest

RUN dnf install -y alsa-lib-devel file unzip https://github.com/charmbracelet/gum/releases/download/v0.13.0/gum-0.13.0-1.x86_64.rpm && \
    dnf clean all

COPY --from=builder /app/goqoa /usr/bin/
COPY --from=builder /app/qoa/qoaconv /usr/bin/
COPY --from=builder /app/compare.sh /app/

ENV TERM=xterm-256color

ENTRYPOINT ["/app/compare.sh"]
