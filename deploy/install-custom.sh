#!/bin/bash
# =============================================================================
# Sub2API 一键安装脚本（二开自建版）
# =============================================================================
# 用途：在一台干净的 Ubuntu/Debian 服务器上，从本仓库源码构建并启动 Sub2API。
#   - 检查/安装 Docker
#   - 首次运行自动生成 .env（含随机 POSTGRES_PASSWORD / JWT_SECRET / TOTP_ENCRYPTION_KEY）
#   - 用 deploy/docker-compose.build.yml 从源码构建镜像并启动全部服务
#
# 用法（在项目根目录执行）：
#   bash deploy/install-custom.sh
# =============================================================================

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

command_exists() { command -v "$1" >/dev/null 2>&1; }

# ---------------------------------------------------------------------------
# 定位项目根目录（脚本位于 deploy/ 下）
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.build.yml"
ENV_FILE="${SCRIPT_DIR}/.env"
ENV_EXAMPLE="${SCRIPT_DIR}/.env.example"

cd "${ROOT_DIR}"

print_info "项目根目录：${ROOT_DIR}"

# ---------------------------------------------------------------------------
# 1. 检查 Docker
# ---------------------------------------------------------------------------
if ! command_exists docker; then
    print_warning "未检测到 Docker，正在自动安装..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker || true
    print_success "Docker 安装完成"
else
    print_success "Docker 已安装：$(docker --version)"
fi

if ! docker compose version >/dev/null 2>&1; then
    print_error "未检测到 docker compose 插件（v2）。请升级 Docker 到较新版本后重试。"
    exit 1
fi

# ---------------------------------------------------------------------------
# 2. 生成 .env（如果不存在）
# ---------------------------------------------------------------------------
gen_secret() {
    if command_exists openssl; then
        openssl rand -hex 32
    else
        head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
    fi
}

if [ -f "${ENV_FILE}" ]; then
    print_warning ".env 已存在，跳过生成（如需重置请手动删除 ${ENV_FILE}）"
else
    print_info "生成 .env 配置文件..."
    cp "${ENV_EXAMPLE}" "${ENV_FILE}"

    PG_PWD="$(gen_secret)"
    JWT="$(gen_secret)"
    TOTP="$(gen_secret)"

    # 写入/替换关键变量（若 .env.example 已有该键则替换，否则追加）
    set_env() {
        local key="$1" val="$2"
        if grep -qE "^${key}=" "${ENV_FILE}"; then
            # 用 | 作分隔符，避免值里的 / 干扰
            sed -i "s|^${key}=.*|${key}=${val}|" "${ENV_FILE}"
        else
            echo "${key}=${val}" >> "${ENV_FILE}"
        fi
    }

    set_env "POSTGRES_PASSWORD" "${PG_PWD}"
    set_env "JWT_SECRET" "${JWT}"
    set_env "TOTP_ENCRYPTION_KEY" "${TOTP}"

    print_success ".env 已生成，并写入随机的数据库密码 / JWT 密钥 / TOTP 密钥"
    print_warning "管理员账号使用代码内置默认值（ADMIN_EMAIL / ADMIN_PASSWORD）。"
    print_warning "如需自定义，请编辑 ${ENV_FILE} 后再继续。"
fi

# ---------------------------------------------------------------------------
# 3. 构建并启动
# ---------------------------------------------------------------------------
print_info "开始构建镜像并启动服务（首次约需 3-8 分钟）..."
docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d --build

# ---------------------------------------------------------------------------
# 4. 状态与访问信息
# ---------------------------------------------------------------------------
echo ""
print_info "当前容器状态："
docker compose -f "${COMPOSE_FILE}" ps

SERVER_PORT="$(grep -E '^SERVER_PORT=' "${ENV_FILE}" | cut -d= -f2 || true)"
SERVER_PORT="${SERVER_PORT:-8080}"
IP_ADDR="$(hostname -I 2>/dev/null | awk '{print $1}' || echo '服务器IP')"

echo ""
print_success "部署完成！"
echo -e "  访问地址：${GREEN}http://${IP_ADDR}:${SERVER_PORT}${NC}"
echo -e "  查看日志：${BLUE}docker compose -f deploy/docker-compose.build.yml logs -f sub2api${NC}"
echo ""
print_warning "云服务器请在安全组/防火墙放行 ${SERVER_PORT} 端口。"
print_warning "首次登录后请尽快在后台修改管理员密码。"
