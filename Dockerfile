FROM quay.io/derailed/k9s:latest

RUN apk add openssh curl openssl
RUN mkdir /workshop
RUN curl -sL https://run.linkerd.io/install | sh

ENV PATH=${PATH}:/root/.linkerd2/bin
ENV KUBECONFIG=/workshop/k3s.yaml

WORKDIR /workshop

ENTRYPOINT [ "ash" ]