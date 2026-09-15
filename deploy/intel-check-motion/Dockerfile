FROM mcr.microsoft.com/playwright:v1.63.0-noble

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --omit=dev

COPY motion.mjs server.mjs stdin.mjs worker.mjs ./

ENV PORT=8081
USER pwuser
EXPOSE 8081

CMD ["node", "server.mjs"]
