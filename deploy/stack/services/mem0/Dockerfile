FROM docker.m.daocloud.io/library/python:3.11-slim

WORKDIR /app

RUN sed -i 's|http://deb.debian.org/debian|http://mirrors.aliyun.com/debian|g' /etc/apt/sources.list.d/debian.sources && \
    sed -i 's|http://security.debian.org/debian-security|http://mirrors.aliyun.com/debian-security|g' /etc/apt/sources.list.d/debian.sources && \
    apt-get update && apt-get install -y --no-install-recommends git libpq-dev gcc sed postgresql-client && rm -rf /var/lib/apt/lists/*

ADD https://github.com/mem0ai/mem0/archive/refs/heads/main.tar.gz /tmp/mem0.tar.gz
RUN tar -xzf /tmp/mem0.tar.gz -C /tmp && rm -rf /app && mv /tmp/mem0-main /app && rm /tmp/mem0.tar.gz

RUN pip install --no-cache-dir -e /app
RUN pip install --no-cache-dir -r server/requirements.txt
RUN pip install --no-cache-dir ollama

COPY patch_main.py /tmp/patch_main.py
RUN python3 /tmp/patch_main.py

WORKDIR /app/server

COPY start-server.sh /app/server/start.sh
RUN chmod +x /app/server/start.sh

EXPOSE 8000

CMD ["/app/server/start.sh"]