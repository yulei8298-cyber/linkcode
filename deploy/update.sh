#!/bin/bash
# =============================================================================
# Sub2API 一键更新脚本（二开自建版）
# =============================================================================
# 用途：在服务器上拉取最新代码（git 部署时）、重新构建镜像并滚动重启。
#   - 数据库 / Redis 不受影响，数据不丢
#   - 自动备份数据库（除非加 --no-backup）
#
# 用法（在项目根目录执行）：
#   bash deploy/update.sh              # 标准更新：git pull + 重建 + 重启
#   bash deploy/update.sh --no-pull    # 不执行 git pull（适合手动上传代码的场景）
#   bash deploy/update.sh --no-backup  # 跳过数据库备份
#   注意：请用 bash 运行，不要用 sh（sh 在 Debian/Ubuntu 是 dash，不支持本脚本语法）
# =============================================================================

# 若被 sh/dash 等非 bash 解释器调用，自动用 bash 重新执行自己
if [ -z "${BASH_VERSION:-}" ]; then
    exec bash "$0" "$@"
fi

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[OK]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARN]${NC} $1"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

# 解析参数
DO_PULL=1
DO_BACKUP=1
for arg in "$@"; do
    case "$arg" in
        --no-pull)   DO_PULL=0 ;;
        --no-backup) DO_BACKUP=0 ;;
        *) print_warning "未知参数：$arg" ;;
    esac
done

resolve_deploy_dir() {
    local src dir
    src="${BASH_SOURCE[0]:-$0}"
    dir="$(cd "$(dirname "${src}")" 2>/dev/null && pwd)"
    if [ -n "${dir}" ] && [ -f "${dir}/docker-compose.build.yml" ]; then
        echo "${dir}"; return 0
    fi
    if [ -f "./deploy/docker-compose.build.yml" ]; then
        (cd ./deploy && pwd); return 0
    fi
    if [ -f "./docker-compose.build.yml" ]; then
        pwd; return 0
    fi
    return 1
}

SCRIPT_DIR="$(resolve_deploy_dir)" || {
    print_error "找不到 docker-compose.build.yml。请在项目根目录执行：bash deploy/update.sh"
    exit 1
}
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.build.yml"
ENV_FILE="${SCRIPT_DIR}/.env"

cd "${ROOT_DIR}"

if [ ! -f "${ENV_FILE}" ]; then
    print_error "未找到 ${ENV_FILE}，请先运行 deploy/install-custom.sh 完成首次安装。"
    exit 1
fi

# ---------------------------------------------------------------------------
# 1. 拉取最新代码
# ---------------------------------------------------------------------------
if [ "${DO_PULL}" -eq 1 ]; then
    if [ -d "${ROOT_DIR}/.git" ]; then
        print_info "拉取最新代码（git pull）..."
        git pull
        print_success "代码已更新"
    else
        print_warning "当前目录不是 git 仓库，跳过 git pull（请确认已手动上传新代码）"
    fi
else
    print_info "已指定 --no-pull，跳过 git pull"
fi

# ---------------------------------------------------------------------------
# 2. 备份数据库
# ---------------------------------------------------------------------------
if [ "${DO_BACKUP}" -eq 1 ]; then
    # 仅当 postgres 容器在运行时才备份
    if docker compose -f "${COMPOSE_FILE}" ps postgres 2>/dev/null | grep -q "Up\|running"; then
        PG_USER="$(grep -E '^POSTGRES_USER=' "${ENV_FILE}" | cut -d= -f2 || true)"
        PG_USER="${PG_USER:-sub2api}"
        PG_DB="$(grep -E '^POSTGRES_DB=' "${ENV_FILE}" | cut -d= -f2 || true)"
        PG_DB="${PG_DB:-sub2api}"
        BACKUP_DIR="${ROOT_DIR}/backups"
        mkdir -p "${BACKUP_DIR}"
        BACKUP_FILE="${BACKUP_DIR}/db_$(date +%Y%m%d_%H%M%S).sql"
        print_info "备份数据库到 ${BACKUP_FILE} ..."
        if docker compose -f "${COMPOSE_FILE}" exec -T postgres \
            pg_dump -U "${PG_USER}" "${PG_DB}" > "${BACKUP_FILE}" 2>/dev/null; then
            print_success "数据库已备份"
        else
            print_warning "数据库备份失败（可能是首次部署尚无数据），继续更新"
            rm -f "${BACKUP_FILE}"
        fi
    else
        print_info "postgres 容器未运行，跳过备份"
    fi
else
    print_info "已指定 --no-backup，跳过数据库备份"
fi

# ---------------------------------------------------------------------------
# 3. 重新构建并滚动重启
# ---------------------------------------------------------------------------
print_info "重新构建镜像并滚动重启..."
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d --build

# ---------------------------------------------------------------------------
# 4. 清理悬空镜像 + 状态
# ---------------------------------------------------------------------------
print_info "清理无用的旧镜像..."
docker image prune -f >/dev/null 2>&1 || true

echo ""
print_info "当前容器状态："
docker compose -f "${COMPOSE_FILE}" ps

echo ""
print_success "更新完成！"
echo -e "  查看日志：${BLUE}docker compose -f deploy/docker-compose.build.yml logs -f sub2api${NC}"
