### Pull CA Certs
FROM --platform=$BUILDPLATFORM hub.zentao.net/ci/alpine:3.19 AS cert-image

RUN apk --update add ca-certificates

FROM --platform=$TARGETPLATFORM hub.zentao.net/ci/alpine:3.19

ARG TZ="Asia/Shanghai"
ARG TARGETARCH

ENV TZ=${TZ}

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories \
    && apk --no-cache add git bash tzdata curl subversion \
    && ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime \
    && echo ${TZ} > /etc/timezone \
    && date \
    && rm -rf /var/cache/apk/*

# setup app dir and its content
WORKDIR /app

VOLUME /data

# 安装根目录
ENV GITFOX_WORKSPACE_ROOT_DIR=/data/gitfox-community
# 数据库连接
ENV GITFOX_DB_HOST=127.0.0.1
ENV GITFOX_DB_PORT=3306
ENV GITFOX_DB_USER=root
ENV GITFOX_DB_PASSWORD=please_change
ENV GITFOX_DB_NAME=zentao
ENV GITFOX_DB_TIMEZONE=Asia/Shanghai
# GaussDB 连接示例（openGauss 兼容 PostgreSQL 协议）
ENV GITFOX_DB_DRIVER=mysql
#ENV GITFOX_DB_DATASOURCE=host=127.0.0.1 port=8000 user=gaussdb password=gaussdb dbname=gitfox-community sslmode=disable

#ENV GITFOX_SERVER_PROTO=http
# 对外域名/ip
#ENV GITFOX_SERVER_HOST=example.com

# 代码库地址是否暴露端口
#ENV GITFOX_REPO_URL_EXPOSE_PORT=true

# GitFox http端口
ENV GITFOX_HTTP_PORT=3000

# GitFox ssh端口
ENV GITFOX_SSH_PORT=3322

# 禅道访问 域名+/api.php/v1
#ENV GITFOX_DEVOPS_URL=http://xxx/api.php/v1

# 日志输出的最低级别  -1~5：debug,info,warn,error,dpanic,panic,fatal
ENV GITFOX_LOG_LEVEL=0
# Docker daemon 连接配置：默认连接容器内 /var/run/docker.sock（SDK 默认值）。
# 远端 daemon 可用 docker run -e GITFOX_DOCKER_HOST=... -e GITFOX_DOCKER_TLS_VERIFY=1
# -e GITFOX_DOCKER_CERT_PATH=... 覆盖；兼容标准 DOCKER_HOST，但 GITFOX_DOCKER_HOST 优先。
# 不在这里写死 ENV：镜像 ENV 会覆盖 prod.env 中的同名配置。

ENV GITFOX_CI_PLUGINS_ZIP_URL=/app/ciPlugins/plugins.tar.gz

COPY bin/linux-${TARGETARCH}/gitfox /app/gitfox

# 默认配置模板：仅用于运行期播种到 /data
COPY bin/linux-${TARGETARCH}/prod.env /app/defaults/prod.env
COPY bin/linux-${TARGETARCH}/prod.yaml /app/defaults/prod.yaml

COPY resources/ciPlugins/plugins.tar.gz /app/ciPlugins/plugins.tar.gz



COPY --from=cert-image /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY deploy/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 3000
EXPOSE 22
EXPOSE 3322

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["server", "/data/gitfox-community/prod"]
