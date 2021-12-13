FROM centos:7

LABEL maintainer="brewlin" version="1.0" license="MIT"

RUN yum install -y gcc-c++

ENV PATH /usr/local/go/bin:$PATH
ENV GOROOT /usr/local/go
ENV GOPATH /home/go

RUN yum -y install wget \
    && mkdir /home/go \
    && wget https://studygolang.com/dl/golang/go1.13.10.linux-amd64.tar.gz \
    && tar -C /usr/local -zxf go1.13.10.linux-amd64.tar.gz \
    && yum -y install iproute net-tools

RUN echo export GOROOT=/usr/local/go >> /etc/profile
RUN echo export GOPATH=/home/go >> /etc/profile
RUN echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/profile
RUN rm -f go1.13.10.linux-amd64.tar.gz
RUN source /etc/profile && go version
RUN /bin/cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo 'Asia/Shanghai' >/etc/timezone

ADD . /test/net-protocol

RUN cd /test/net-protocol \
    && cd tool            \
    && go build -x up.go  \
    && ./up

WORKDIR /test/net-protocol

CMD [ "/test/net-protocol/tool/up"]