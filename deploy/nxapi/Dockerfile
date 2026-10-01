# The nxapi sidecar of the Nintendo connector: it speaks the Nintendo Switch
# Online app's protocol ("f" token, encrypted requests) on behalf of the KFIRE
# server, which reaches it on the internal network only. Pinned on purpose:
# Nintendo app updates break older nxapi releases, and moving to a new one is
# a deliberate change (bump the version, rebuild, redeploy).
FROM node:22-alpine
RUN npm install -g nxapi@1.6.1-next.257 && npm cache clean --force
COPY entrypoint.mjs /entrypoint.mjs
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["node", "/entrypoint.mjs"]
