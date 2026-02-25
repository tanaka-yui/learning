FROM composer:2 AS builder

WORKDIR /app

RUN composer create-project laravel/laravel . --prefer-dist --no-interaction --no-dev

COPY routes/web.php /app/routes/web.php
COPY app/Http/Controllers/HeavyController.php /app/app/Http/Controllers/HeavyController.php

RUN php artisan key:generate && \
    php artisan config:clear && \
    php artisan route:clear

FROM php:8.4-fpm

ENV SESSION_DRIVER=array
ENV APP_DEBUG=false

RUN apt-get update && apt-get install -y --no-install-recommends \
    nginx \
    supervisor \
    gettext-base \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /var/www/html

COPY --from=builder /app /var/www/html

RUN chown -R www-data:www-data /var/www/html/storage /var/www/html/bootstrap/cache

COPY nginx.conf /etc/nginx/sites-available/default
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY www.conf /usr/local/etc/php-fpm.d/www.conf.template
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
