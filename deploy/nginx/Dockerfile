# Kontainer nginx reverse proxy. Build context: folder deploy/nginx/
# nginx.conf = server HTTP; server.conf = isi server bersama.
FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/nginx.conf
COPY server.conf /etc/nginx/snippets/server.conf

EXPOSE 80
