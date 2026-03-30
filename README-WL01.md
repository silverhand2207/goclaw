# Init env
./prepare-env.sh

# Start docker 
docker compose -f docker-compose.yml -f docker-compose.postgres.yml -f docker-compose.selfservice.yml -f docker-compose.claude-cli.yml -f docker-compose.traefik.yml up -d --build