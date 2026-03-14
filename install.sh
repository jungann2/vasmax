#!/usr/bin/env bash
set -euo pipefail

# VasmaX 部署脚本
# 功能：系统检测、依赖安装、Go 二进制下载与校验、systemd 服务配置、
#       TLS 证书管理（acme.sh）、Nginx 安装、卸载清理
readonly SCRIPT_VERSION="1.0.0"
readonly BINARY_NAME="VasmaX"
readonly INSTALL_PATH="/usr/local/bin/${BINARY_NAME}"
readonly CONFIG_DIR="/etc/vasmax"
readonly CONFIG_FILE="${CONFIG_DIR}/config.yaml"
readonly LOG_DIR="/var/log/vasmax"
readonly SERVICE_FILE="/etc/systemd/system/VasmaX.service"
readonly TLS_DIR="${CONFIG_DIR}/tls"
readonly GITHUB_REPO="jungann2/vasmax"

# --- 颜色输出 ---
red() { echo -e "\033[31m$1\033[0m"; }
green() { echo -e "\033[32m$1\033[0m"; }
yellow() { echo -e "\033[33m$1\033[0m"; }

# --- 系统检测 ---
detect_os() {
    if [[ -f /etc/os-release ]]; then
        # 用 grep 提取，避免 source 与脚本 readonly 变量冲突
        OS_TYPE="$(grep -oP '^ID=\K\S+' /etc/os-release | tr -d '"')"
        OS_VERSION="$(grep -oP '^VERSION_ID=\K\S+' /etc/os-release 2>/dev/null | tr -d '"' || echo "")"
    elif command -v lsb_release &>/dev/null; then
        OS_TYPE="$(lsb_release -si | tr '[:upper:]' '[:lower:]')"
    else
        OS_TYPE="unknown"
    fi

    ARCH="$(uname -m)"
    case "${ARCH}" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) red "不支持的 CPU 架构: ${ARCH}"; exit 1 ;;
    esac

    echo "系统: ${OS_TYPE} ${OS_VERSION:-} (${ARCH})"
}

# --- 依赖安装 ---
install_deps() {
    echo "安装系统依赖..."
    case "${OS_TYPE}" in
        debian|ubuntu)
            apt-get update -y
            apt-get install -y curl wget jq socat unzip cron
            ;;
        centos|rhel|fedora|rocky|almalinux)
            yum install -y curl wget jq socat unzip cronie
            ;;
        alpine)
            apk add --no-cache curl wget jq socat unzip
            ;;
        *)
            yellow "未知系统类型，请手动安装依赖: curl wget jq socat unzip"
            ;;
    esac
}

# --- 下载二进制 ---
download_binary() {
    local version="${1:-latest}"
    local download_url
    local sha256_url

    if [[ "${version}" == "latest" ]]; then
        download_url="https://github.com/${GITHUB_REPO}/releases/latest/download/${BINARY_NAME}_linux_${ARCH}"
        sha256_url="${download_url}.sha256"
    else
        download_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${BINARY_NAME}_linux_${ARCH}"
        sha256_url="${download_url}.sha256"
    fi

    echo "下载 ${BINARY_NAME}..."
    local tmp_file
    tmp_file="$(mktemp)"

    if ! curl -fsSL -o "${tmp_file}" "${download_url}"; then
        rm -f "${tmp_file}"
        red "下载失败: ${download_url}"
        exit 1
    fi

    # SHA256 校验
    echo "校验 SHA256..."
    local expected_hash
    if expected_hash="$(curl -fsSL "${sha256_url}" 2>/dev/null)" && [[ -n "${expected_hash}" ]]; then
        expected_hash="$(echo "${expected_hash}" | awk '{print $1}')"
        local actual_hash
        actual_hash="$(sha256sum "${tmp_file}" | awk '{print $1}')"
        if [[ "${expected_hash}" != "${actual_hash}" ]]; then
            rm -f "${tmp_file}"
            red "SHA256 校验失败"
            red "期望: ${expected_hash}"
            red "实际: ${actual_hash}"
            exit 1
        fi
        green "SHA256 校验通过"
    else
        yellow "SHA256 校验文件不可用，跳过校验"
    fi

    # 安装二进制
    mv "${tmp_file}" "${INSTALL_PATH}"
    chmod 755 "${INSTALL_PATH}"
    green "已安装到 ${INSTALL_PATH}"
}

# --- systemd 服务 ---
setup_service() {
    # Alpine 使用 OpenRC，不支持 systemd
    if [[ "${OS_TYPE}" == "alpine" ]]; then
        yellow "Alpine 系统检测到，跳过 systemd 配置"
        yellow "请手动配置 OpenRC 服务或直接运行: /usr/local/bin/VasmaX -c /etc/vasmax/config.yaml"

        # 仍然安装 vasmax 命令别名
        cat > /usr/local/bin/vasmax << 'ALIAS'
#!/usr/bin/env bash
/usr/local/bin/VasmaX --menu "$@"
ALIAS
        chmod 755 /usr/local/bin/vasmax
        return
    fi

    echo "配置 systemd 服务..."
    cat > "${SERVICE_FILE}" << 'EOF'
[Unit]
Description=VasmaX - Proxy Node Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/VasmaX -c /etc/vasmax/config.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable VasmaX

    # 安装 vasmax 命令别名（小写命令指向大写二进制）
    cat > /usr/local/bin/vasmax << 'ALIAS'
#!/usr/bin/env bash
/usr/local/bin/VasmaX --menu "$@"
ALIAS
    chmod 755 /usr/local/bin/vasmax

    green "systemd 服务已配置"
}

# --- 写入默认配置 ---
write_default_config() {
    cat > "${CONFIG_FILE}" << YAML
standalone: true
log:
  level: info
  file_path: /var/log/vasmax/VasmaX.log
audit:
  enabled: true
  file_path: /var/log/vasmax/audit.log
  max_size: 50
  max_files: 3
lang: zh
protocols: []
core_type: dual
monitoring_enabled: true
YAML
    chmod 600 "${CONFIG_FILE}"
}

# --- 初始化目录 ---
init_dirs() {
    mkdir -p "${CONFIG_DIR}" "${CONFIG_DIR}/xray/conf" "${CONFIG_DIR}/sing-box/conf/config"
    mkdir -p "${CONFIG_DIR}/subscribe" "${CONFIG_DIR}/subscribe_local" "${CONFIG_DIR}/subscribe_remote"
    mkdir -p "${CONFIG_DIR}/cache" "${CONFIG_DIR}/tls"
    mkdir -p "${LOG_DIR}"
    chmod 700 "${CONFIG_DIR}"
    chmod 700 "${TLS_DIR}"

    # 配置文件处理
    if [[ -f "${CONFIG_FILE}" ]]; then
        yellow "检测到已有配置文件: ${CONFIG_FILE}"
        echo "1) 保留当前配置（推荐）"
        echo "2) 备份当前配置并重置为默认"
        echo "3) 查看当前配置"
        read -rp "请选择 [1-3]（默认 1）: " config_choice
        case "${config_choice}" in
            2)
                local backup_file="${CONFIG_FILE}.bak.$(date +%Y%m%d%H%M%S)"
                cp "${CONFIG_FILE}" "${backup_file}"
                green "已备份到 ${backup_file}"
                write_default_config
                green "配置已重置为默认"
                ;;
            3)
                echo "─────────────────────────────"
                cat "${CONFIG_FILE}"
                echo "─────────────────────────────"
                read -rp "是否重置为默认配置? [y/N]: " reset_choice
                case "${reset_choice}" in
                    [yY]|[yY][eE][sS])
                        local backup_file="${CONFIG_FILE}.bak.$(date +%Y%m%d%H%M%S)"
                        cp "${CONFIG_FILE}" "${backup_file}"
                        green "已备份到 ${backup_file}"
                        write_default_config
                        green "配置已重置为默认"
                        ;;
                    *)
                        green "保留当前配置"
                        ;;
                esac
                ;;
            *)
                green "保留当前配置"
                ;;
        esac
    else
        write_default_config
    fi
}

# --- TLS 证书管理 ---
install_acme() {
    if command -v ~/.acme.sh/acme.sh &>/dev/null; then
        green "acme.sh 已安装"
        return
    fi
    echo "安装 acme.sh..."
    curl -fsSL https://get.acme.sh | sh -s email=admin@example.com
}

issue_cert() {
    local domain="${1}"
    local provider="${2:-letsencrypt}"
    local mode="${3:-standalone}"

    install_acme

    local ca_server
    case "${provider}" in
        letsencrypt) ca_server="--server letsencrypt" ;;
        buypass) ca_server="--server buypass" ;;
        zerossl) ca_server="--server zerossl" ;;
        *) ca_server="--server letsencrypt" ;;
    esac

    echo "申请证书: ${domain} (${provider}, ${mode})"

    case "${mode}" in
        standalone)
            ~/.acme.sh/acme.sh --issue -d "${domain}" --standalone ${ca_server}
            ;;
        dns_cf)
            # Cloudflare DNS API
            if [[ -z "${CF_Token:-}" ]]; then
                red "请设置 CF_Token 环境变量"
                return 1
            fi
            export CF_Token
            ~/.acme.sh/acme.sh --issue -d "${domain}" --dns dns_cf ${ca_server}
            ;;
        dns_ali)
            # 阿里云 DNS API
            if [[ -z "${Ali_Key:-}" ]] || [[ -z "${Ali_Secret:-}" ]]; then
                red "请设置 Ali_Key 和 Ali_Secret 环境变量"
                return 1
            fi
            export Ali_Key Ali_Secret
            ~/.acme.sh/acme.sh --issue -d "${domain}" --dns dns_ali ${ca_server}
            ;;
        dns_cf_wildcard)
            # 通配符证书
            if [[ -z "${CF_Token:-}" ]]; then
                red "请设置 CF_Token 环境变量"
                return 1
            fi
            export CF_Token
            ~/.acme.sh/acme.sh --issue -d "${domain}" -d "*.${domain}" --dns dns_cf ${ca_server}
            ;;
    esac

    # 安装证书，续期后自动重启 VasmaX 服务
    ~/.acme.sh/acme.sh --install-cert -d "${domain}" \
        --cert-file "${TLS_DIR}/${domain}.crt" \
        --key-file "${TLS_DIR}/${domain}.key" \
        --fullchain-file "${TLS_DIR}/${domain}.fullchain.crt" \
        --reloadcmd "systemctl restart VasmaX"

    chmod 600 "${TLS_DIR}/${domain}.key"

    # 确保 acme.sh cron job 已配置（自动续期）
    if ! crontab -l 2>/dev/null | grep -q "acme.sh"; then
        ~/.acme.sh/acme.sh --install-cronjob
        green "已配置 acme.sh 自动续期 cron job"
    fi

    green "证书已安装到 ${TLS_DIR}/"
    green "证书续期后将自动重启 VasmaX 服务"
}

# --- Nginx 安装 ---
install_nginx() {
    if command -v nginx &>/dev/null; then
        green "Nginx 已安装"
        return
    fi

    echo "安装 Nginx..."
    case "${OS_TYPE}" in
        debian|ubuntu)
            apt-get install -y nginx
            ;;
        centos|rhel|fedora|rocky|almalinux)
            yum install -y nginx
            ;;
        alpine)
            apk add --no-cache nginx
            ;;
    esac

    if [[ "${OS_TYPE}" == "alpine" ]]; then
        rc-update add nginx default 2>/dev/null || true
        rc-service nginx start 2>/dev/null || true
    else
        systemctl enable nginx
        systemctl start nginx
    fi
    green "Nginx 已安装并启动"
}

# --- 卸载 Nginx ---
uninstall_nginx() {
    if ! command -v nginx &>/dev/null; then
        return
    fi

    echo ""
    read -rp "是否同时卸载 Nginx? (如有其他站点使用请选否) [y/N]: " nginx_choice
    case "${nginx_choice}" in
        [yY]|[yY][eE][sS])
            yellow "正在卸载 Nginx..."
            systemctl stop nginx 2>/dev/null || true
            systemctl disable nginx 2>/dev/null || true
            case "${OS_TYPE}" in
                debian|ubuntu)
                    apt-get purge -y nginx nginx-common nginx-full 2>/dev/null || true
                    apt-get autoremove -y 2>/dev/null || true
                    ;;
                centos|rhel|fedora|rocky|almalinux)
                    yum remove -y nginx 2>/dev/null || true
                    ;;
                alpine)
                    apk del nginx 2>/dev/null || true
                    ;;
            esac
            green "Nginx 已卸载"
            ;;
        *)
            yellow "保留 Nginx"
            ;;
    esac
}

# --- 卸载 ---
uninstall() {
    yellow "开始卸载 VasmaX..."

    # 检测系统类型（卸载 Nginx 需要）
    detect_os

    # 停止服务
    systemctl stop VasmaX 2>/dev/null || true
    systemctl disable VasmaX 2>/dev/null || true

    # 删除文件
    rm -f "${INSTALL_PATH}"
    rm -f "${SERVICE_FILE}"
    rm -f /usr/local/bin/vasmax

    # 删除配置和数据（需确认）
    if [[ "${1:-}" == "--purge" ]]; then
        rm -rf "${CONFIG_DIR}"
        rm -rf "${LOG_DIR}"
        green "配置和数据已清除"
    else
        yellow "配置保留在 ${CONFIG_DIR}，使用 --purge 完全清除"
    fi

    # 询问是否卸载 Nginx
    uninstall_nginx

    systemctl daemon-reload
    green "VasmaX 已卸载"
    yellow "注意: acme.sh 证书配置未被移除"
    yellow "如需清理请手动执行: ~/.acme.sh/acme.sh --uninstall"
}

# --- 主菜单 ---
show_menu() {
    echo ""
    green "VasmaX 部署脚本 v${SCRIPT_VERSION}"
    echo "─────────────────────────────"
    echo " 1. 安装 VasmaX"
    echo " 2. 更新 VasmaX"
    echo " 3. 卸载 VasmaX"
    echo " 4. 启动服务"
    echo " 5. 停止服务"
    echo " 6. 重启服务"
    echo " 7. 查看状态"
    echo " 8. 查看日志"
    echo " 9. 申请 TLS 证书"
    echo " 0. 退出"
    echo ""
    read -rp "请选择 [0-9]: " choice

    case "${choice}" in
        1) do_install ;;
        2) do_update ;;
        3)
            echo "1) 保留配置卸载  2) 完全清除（含配置和数据）"
            read -rp "选择 [1-2]: " uninstall_choice
            case "${uninstall_choice}" in
                2) uninstall "--purge" ;;
                *) uninstall ;;
            esac
            ;;
        4) systemctl start VasmaX && green "已启动" ;;
        5) systemctl stop VasmaX && green "已停止" ;;
        6) systemctl restart VasmaX && green "已重启" ;;
        7) systemctl status VasmaX ;;
        8) journalctl -u VasmaX -f --no-pager ;;
        9) cert_menu ;;
        0) exit 0 ;;
        *) red "无效选择" ;;
    esac
}

cert_menu() {
    read -rp "请输入域名: " domain
    echo "证书提供商: 1) Let's Encrypt  2) Buypass  3) ZeroSSL"
    read -rp "选择 [1-3]: " provider_choice
    case "${provider_choice}" in
        1) provider="letsencrypt" ;;
        2) provider="buypass" ;;
        3) provider="zerossl" ;;
        *) provider="letsencrypt" ;;
    esac
    echo "验证方式: 1) standalone  2) Cloudflare DNS  3) 阿里云 DNS  4) CF 通配符"
    read -rp "选择 [1-4]: " mode_choice
    case "${mode_choice}" in
        1) mode="standalone" ;;
        2) mode="dns_cf" ;;
        3) mode="dns_ali" ;;
        4) mode="dns_cf_wildcard" ;;
        *) mode="standalone" ;;
    esac
    issue_cert "${domain}" "${provider}" "${mode}"
}

do_install() {
    detect_os
    install_deps
    install_nginx
    init_dirs
    download_binary
    setup_service
    if [[ "${OS_TYPE}" != "alpine" ]]; then
        systemctl start VasmaX
    fi
    green "安装完成！运行 vasmax 打开管理菜单"
}

do_update() {
    detect_os
    if [[ "${OS_TYPE}" != "alpine" ]]; then
        systemctl stop VasmaX 2>/dev/null || true
    fi
    download_binary
    if [[ "${OS_TYPE}" != "alpine" ]]; then
        systemctl start VasmaX
    fi
    green "更新完成"
}

# --- 入口 ---
main() {
    if [[ $EUID -ne 0 ]]; then
        red "请使用 root 用户运行此脚本"
        exit 1
    fi

    case "${1:-}" in
        install) do_install ;;
        update) do_update ;;
        uninstall) uninstall "${2:-}" ;;
        start) systemctl start VasmaX ;;
        stop) systemctl stop VasmaX ;;
        restart) systemctl restart VasmaX ;;
        status) systemctl status VasmaX ;;
        log) journalctl -u VasmaX -f --no-pager ;;
        cert) shift; issue_cert "$@" ;;
        *) show_menu ;;
    esac
}

main "$@"
