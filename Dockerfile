FROM golang:1.4

RUN go get -v github.com/constabulary/gb/...

WORKDIR /usr/src/dtodo
ENV PATH /usr/src/dtodo/bin:$PATH
COPY . /usr/src/dtodo

RUN gb build
