# GoClaw — WL01 Setup

## Init env

```bash
./prepare-env.sh
```

## Compose stack (WL01)

Stack mặc định gồm 5 overlay:

```bash
COMPOSE="-f docker-compose.yml \
  -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml \
  -f docker-compose.claude-cli.yml \
  -f docker-compose.traefik.yml"
```

## Start

```bash
docker compose $COMPOSE up -d --build
```

## Upgrade

Quy trình upgrade gồm 3 bước: pull source mới → backup DB → apply schema migrations + rebuild containers.

### 1. Pull source mới (submodule `wl01`)

```bash
# Từ thư mục parent infras/
git submodule update --remote --merge goclaw

# Hoặc thủ công bên trong submodule
cd goclaw && git fetch origin && git pull --ff-only origin wl01 && cd ..

# Commit con trỏ submodule mới
git add goclaw && git commit -m "chore: bump goclaw submodule"
```

### 2. Backup database (khuyến nghị)

```bash
cd goclaw
docker compose -f docker-compose.postgres.yml exec -T postgres \
  pg_dump -U goclaw goclaw | gzip > backup-$(date +%Y%m%d-%H%M).sql.gz
```

### 3. Apply schema upgrade + rebuild

```bash
# Xem trạng thái schema hiện tại
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.upgrade.yml run --rm upgrade --status

# Dry-run (không apply)
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.upgrade.yml run --rm upgrade --dry-run

# Apply migrations + data hooks (idempotent, build image mới luôn)
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.upgrade.yml run --rm --build upgrade

# Restart stack với image mới
docker compose $COMPOSE up -d --build
```

## Auto-upgrade khi start (tùy chọn)

Thêm vào `.env` để gateway tự chạy migrations lúc khởi động (bỏ qua bước 3):

```env
GOCLAW_AUTO_UPGRADE=true
```

## Lệnh thường dùng

```bash
# Logs
docker compose $COMPOSE logs -f goclaw

# Restart 1 service
docker compose $COMPOSE restart goclaw

# Stop toàn bộ stack
docker compose $COMPOSE down

# Reset sạch (XOÁ volume — mất data!)
docker compose $COMPOSE down -v
```

## Rollback

Nếu upgrade fail và schema bị `dirty`:

```bash
# Xem chi tiết lỗi
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.upgrade.yml run --rm upgrade --status

# Restore từ backup
gunzip -c backup-YYYYMMDD-HHMM.sql.gz | \
  docker compose -f docker-compose.postgres.yml exec -T postgres \
  psql -U goclaw goclaw
```
