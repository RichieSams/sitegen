FROM alpine:3.14.2

RUN apk add --no-cache tini rsync

ADD sitegen /usr/local/bin/
ADD docker-entrypoint.sh /
RUN chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/sbin/tini", "-g", "--", "/docker-entrypoint.sh"]
CMD ["serve"]
