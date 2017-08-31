FROM scratch
MAINTAINER Peyton Vaughn <pvaughn@6fusion.com>

WORKDIR /app

COPY sidecar /app/

CMD ["/app/sidecar"]


