#!/usr/bin/env bash
set -euo pipefail

export PHP_FPM_WORKERS="${PHP_FPM_WORKERS:-4}"

envsubst '${PHP_FPM_WORKERS}' < /usr/local/etc/php-fpm.d/www.conf.template > /usr/local/etc/php-fpm.d/www.conf

exec /usr/bin/supervisord -c /etc/supervisor/conf.d/supervisord.conf
