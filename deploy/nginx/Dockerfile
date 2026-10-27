# Kontainer nginx (reverse proxy). Build context: folder deploy/nginx/
# Konfigurasi dibuat dari templates/ saat container start, lewat envsubst bawaan image nginx.
#
# Variabel saat run:
#   QMS_DOMAIN       nama host, mis. qms.contoh.ac.id (default localhost)
#   BACKEND_HOST     nama service backend di jaringan Docker (default backend)
#   QR_SERVICE_HOST  nama service qr-service di jaringan Docker (default qr-service)
#   FRONTEND_ROOT    folder frontend statis di dalam kontainer (default /srv/qms-frontend)
FROM nginx:1.27-alpine

COPY templates/ /etc/nginx/templates/

ENV QMS_DOMAIN=localhost \
    BACKEND_HOST=backend \
    QR_SERVICE_HOST=qr-service \
    FRONTEND_ROOT=/srv/qms-frontend

EXPOSE 80
