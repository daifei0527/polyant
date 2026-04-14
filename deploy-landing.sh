#!/bin/bash
# AgentWiki Landing Page Deployment Script

# 创建网站目录
mkdir -p /var/www/openclaw.dlibrary.cn

# 复制页面文件
cp /home/daifei/agentwiki/web/landing/index.html /var/www/openclaw.dlibrary.cn/

# 设置权限
chown -R www-data:www-data /var/www/openclaw.dlibrary.cn

# 更新 nginx 配置
cat > /etc/nginx/sites-available/openclaw.dlibrary.cn.conf << 'NGINX_CONF'
server {
    server_name openclaw.dlibrary.cn;
    root /var/www/openclaw.dlibrary.cn;
    index index.html;

    access_log /var/log/nginx/openclaw.dlibrary.cn.access.log;
    error_log /var/log/nginx/openclaw.dlibrary.cn.error.log;

    location / {
        try_files $uri $uri/ =404;
    }

    # 缓存静态资源
    location ~* \.(css|js|png|jpg|jpeg|gif|ico|svg|woff|woff2)$ {
        expires 7d;
        add_header Cache-Control "public, immutable";
    }

    listen 443 ssl;
    ssl_certificate /etc/letsencrypt/live/openclaw.dlibrary.cn/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/openclaw.dlibrary.cn/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;
}

server {
    if ($host = openclaw.dlibrary.cn) {
        return 301 https://$host$request_uri;
    }

    listen 80;
    server_name openclaw.dlibrary.cn;
    return 404;
}
NGINX_CONF

# 测试并重载 nginx
nginx -t && systemctl reload nginx

echo "✅ AgentWiki Landing Page deployed to https://openclaw.dlibrary.cn"
