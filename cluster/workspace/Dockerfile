FROM ubuntu:resolute
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
  && apt-get -y install sudo htop vim tini curl wget dnsutils unzip \
  git zsh jq less nano lsof locales make iputils-ping iproute2 \
  && apt-get autoremove -y && apt-get clean -y && rm -rf /var/lib/apt/lists/* \
  && locale-gen en_US.UTF-8
ENTRYPOINT ["tini", "--"]
CMD ["sleep", "infinity"]