# 1.创建Dockerfile

```
root@wolfan-NUC9V7QNX:~/Docker_Build_image# cat Dockerfile
# 使用 nginx:1.29.1 作为构建基础镜像
FROM nginx:1.29.1 AS build

# 安装构建依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  curl \
  git \
  libpcre3-dev \
  libssl-dev \
  zlib1g-dev \
  ca-certificates \
  libxml2-dev \
  libxslt1-dev \
  pkg-config \
  openssl \
  build-essential \
  libtool \
  libssl-dev \
  libpcre2-dev \
  zlib1g-dev \
  pkg-config \
  wget \
  clang \
  libclang-dev \
  && rm -rf /var/lib/apt/lists/*

# 安装 Rust 和 Cargo
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- --default-toolchain stable -y

# 设置环境变量
ENV PATH="/root/.cargo/bin:$PATH"

# 验证 Rust 和 Cargo 是否安装成功
RUN echo $PATH && ls /root/.cargo/bin && cargo --version


# 下载 Nginx 源代码
RUN curl -fSL https://nginx.org/download/nginx-1.29.1.tar.gz -o nginx.tar.gz \
  && tar -xzvf nginx.tar.gz \
  && cd nginx-1.29.1

RUN git clone https://github.com/openresty/echo-nginx-module.git /tmp/echo-nginx-module \
  && git clone https://github.com/vozlt/nginx-module-vts.git /tmp/nginx-module-vts \
  && git clone https://github.com/openresty/rds-json-nginx-module.git /tmp/rds-json-nginx-module \
  && git clone https://github.com/openresty/memc-nginx-module.git /tmp/memc-nginx-module \
  && git clone https://github.com/yaoweibin/ngx_http_substitutions_filter_module.git /tmp/ngx_http_substitutions_filter_module \
  && git clone https://github.com/openresty/redis2-nginx-module.git /tmp/redis2-nginx-module \
  && git clone https://github.com/openresty/headers-more-nginx-module.git /tmp/headers-more-nginx-module \
  && git clone https://github.com/FRiCKLE/ngx_cache_purge.git /tmp/ngx_cache_purge \
  && git clone https://github.com/nginx/nginx-acme.git /tmp/nginx-acme

# 配置和编译 Nginx
RUN cd nginx-1.29.1 \
  && ./configure --with-compat \
  --with-file-aio \
  --with-threads \
  --with-http_addition_module \
  --with-http_auth_request_module \
  --with-http_dav_module \
  --with-http_flv_module \
  --with-http_gunzip_module \
  --with-http_gzip_static_module \
  --with-http_mp4_module \
  --with-http_random_index_module \
  --with-http_realip_module \
  --with-http_secure_link_module \
  --with-http_slice_module \
  --with-http_ssl_module \
  --with-http_stub_status_module \
  --with-http_sub_module \
  --with-http_v2_module \
  --with-http_v3_module \
  --with-mail \
  --with-mail_ssl_module \
  --with-stream \
  --with-stream_realip_module \
  --with-stream_ssl_module \
  --with-stream_ssl_preread_module \
  --add-dynamic-module=/tmp/echo-nginx-module \
  --add-dynamic-module=/tmp/redis2-nginx-module \
  --add-dynamic-module=/tmp/nginx-module-vts \
  --add-dynamic-module=/tmp/rds-json-nginx-module \
  --add-dynamic-module=/tmp/memc-nginx-module \
  --add-dynamic-module=/tmp/ngx_http_substitutions_filter_module \
  --add-dynamic-module=/tmp/headers-more-nginx-module \
  --add-dynamic-module=/tmp/ngx_cache_purge \
  --add-dynamic-module=/tmp/nginx-acme \
  && make -j$(nproc) modules \
  && mkdir -pv /usr/lib/nginx/modules \
  && cp objs/*.so /usr/lib/nginx/modules/ \
  && cd .. \
  && rm -rf nginx-1.29.1 nginx.tar.gz /tmp/*


# 第二阶段：最小化的运行环境
FROM nginx:1.29.1

# 复制编译好的模块
COPY --from=build /usr/lib/nginx/modules/*.so /usr/lib/nginx/modules/

# 创建 Nginx 的默认目录
RUN mkdir -p /etc/nginx/conf.d /var/log/nginx

# 复制自定义配置文件（可选）
#COPY nginx.conf /etc/nginx/nginx.conf

# 暴露端口
EXPOSE 80 443

# 启动 Nginx
CMD ["nginx", "-g", "daemon off;"]
```

> 构建命令
> 
> root@wolfan-NUC9V7QNX:~/Docker\_Build\_image# docker build -t wolf-nginx-mulit:1.29.1 .  
> \[+\] Building 1.1s (15/15) FINISHED docker:default  
> \=> \[internal\] load build definition from Dockerfile 0.0s  
> \=> => transferring dockerfile: 4.20kB 0.0s  
> \=> \[internal\] load metadata for docker.io/library/nginx:1.29.1 0.9s  
> \=> \[internal\] load .dockerignore 0.0s  
> \=> => transferring context: 2B 0.0s  
> \=> \[build 1/9\] FROM docker.io/library/nginx:1.29.1@sha256:d5f28ef21aabddd098f3dbc21fe5b7a7d7a184720bc07da0b 0.0s  
> \=> CACHED \[build 2/9\] RUN apt-get update && apt-get install -y --no-install-recommends build-essential 0.0s  
> \=> CACHED \[build 3/9\] RUN wget https://github.com/LuaJIT/LuaJIT/archive/refs/tags/v2.1.0-beta3.tar.gz && 0.0s  
> \=> CACHED \[build 4/9\] RUN ls /usr/local/include/luajit-2.1 0.0s  
> \=> CACHED \[build 5/9\] RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- --default-to 0.0s  
> \=> CACHED \[build 6/9\] RUN echo /root/.cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bi 0.0s  
> \=> CACHED \[build 7/9\] RUN curl -fSL https://nginx.org/download/nginx-1.29.1.tar.gz -o nginx.tar.gz && tar 0.0s  
> \=> CACHED \[build 8/9\] RUN git clone https://github.com/openresty/echo-nginx-module.git /tmp/echo-nginx-modu 0.0s  
> \=> CACHED \[build 9/9\] RUN cd nginx-1.29.1 && ./configure --with-compat --with-file-aio --with-threads 0.0s  
> \=> CACHED \[stage-1 2/3\] COPY --from=build /usr/lib/nginx/modules/\*.so /usr/lib/nginx/modules/ 0.0s  
> \=> CACHED \[stage-1 3/3\] RUN mkdir -p /etc/nginx/conf.d /var/log/nginx 0.0s  
> \=> exporting to image 0.0s  
> \=> => exporting layers 0.0s  
> \=> => writing image sha256:eebda8668546569b99b80b701783c788de6be06cceaddfa2a44a88a454c1cdd3 0.0s  
> \=> => naming to docker.io/library/wolf-nginx-mulit:1.29.1 0.0s
> 
> What's Next?
> 
> 1.  1\. Sign in to your Docker account → docker login
>     
> 2.  2\. View a summary of image vulnerabilities and recommendations → docker scout quickview
>     

# 2.为什么要多段构建

多段构建就是为了保持最小的镜像体积(下面就是多段构建和没有多段构建的区别)

```
root@wolfan-NUC9V7QNX:~/Docker_Build_image# docker images |grep 1.29
wolf-nginx-mulit                                                 1.29.1                         eebda8668546   7 hours ago     206MB
wolf-nginx-nomulit                                               1.29.1                         f5fe69dfb6f3   7 hours ago     2.42GB
```

# 3.启动一个nginx看是否有了加载的模块

```
# 启动一个容器并查看ID
root@wolfan-NUC9V7QNX:~/Docker_Build_image# docker run -it -d --name wolf-nginx-mulit wolf-nginx-mulit:1.29.1
f49189c25fe5b80f135df3396098c6019216f89c7d5524d012c68d78522cd777
root@wolfan-NUC9V7QNX:~/Docker_Build_image# docker ps |grep wolf-nginx-mulit
f49189c25fe5   wolf-nginx-mulit:1.29.1                                                 "/docker-entrypoint.…"    10 seconds ago   Up 9 seconds             80/tcp, 443/tcp                                                                                wolf-nginx-mulit
# 进入已经启动的容器
root@wolfan-NUC9V7QNX:~/Docker_Build_image# docker exec -it f49189c25fe5 /bin/bash
# 可以看到所有模块
root@f49189c25fe5:/# ls /usr/lib/nginx/modules/
ngx_http_acme_module.so            ngx_http_image_filter_module-debug.so  ngx_http_rds_json_filter_module.so    ngx_http_xslt_filter_module.so
ngx_http_echo_module.so            ngx_http_image_filter_module.so        ngx_http_redis2_module.so        ngx_stream_geoip_module-debug.so
ngx_http_geoip_module-debug.so        ngx_http_js_module-debug.so           ngx_http_subs_filter_module.so        ngx_stream_geoip_module.so
ngx_http_geoip_module.so        ngx_http_js_module.so               ngx_http_vhost_traffic_status_module.so    ngx_stream_js_module-debug.so
ngx_http_headers_more_filter_module.so    ngx_http_memc_module.so               ngx_http_xslt_filter_module-debug.so    ngx_stream_js_module.so
```

> 因为折腾一天流水线改造，所以功能上我还没有验证，应该没什么问题，待我边整理流水线边把acme这个功能输出给大家！

![image-20250908171958500](https://mmbiz.qpic.cn/mmbiz_png/Yka3dMGcR38vAqWKPtSKeBahWjSs8oqezOQQ7voEXib0wVPlOpIqVNP7ib6eo5h5UpZPw1mhVG8XZyb3IXbpgKEQ/640?wx_fmt=png&from=appmsg&watermark=1#imgIndex=0 "null")

image-20250908171958500
