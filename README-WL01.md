# CWD
```bash
➜  goclaw git:(wl01) pwd
/opt/goclaw
```

# Init env
```bash
./prepare-env.sh
```

# Start docker 
```bash
docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml -f docker-compose.claude-cli.yml -f docker-compose.traefik.yml up -d --build
```
