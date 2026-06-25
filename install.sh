#!/usr/bin/env bash
set -euo pipefail

# VasmaX 部署脚本
# 功能：系统检测、依赖安装、Go 二进制下载与校验、systemd 服务配置、卸载清理
readonly SCRIPT_VERSION="1.0.0"
readonly BINARY_NAME="VasmaX"
readonly INSTALL_PATH="/usr/local/bin/${BINARY_NAME}"
readonly INSTALLER_PATH="/usr/local/bin/install_vasmax.sh"
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
        return 1
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
            return 1
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

install_self_script() {
    local src="${BASH_SOURCE[0]}"
    if [[ -f "${src}" ]]; then
        install -m 700 "${src}" "${INSTALLER_PATH}"
        green "安装脚本已保存到 ${INSTALLER_PATH}"
    else
        yellow "无法保存安装脚本自身，请保留当前 install.sh 以便后续更新/卸载"
    fi
}

write_config_reference() {
    if [[ -x "${INSTALL_PATH}" ]]; then
        "${INSTALL_PATH}" -c "${CONFIG_FILE}" --write-config-reference >/dev/null 2>&1 \
            && green "配置参考文件已生成: ${CONFIG_DIR}/config.reference.yaml" \
            || yellow "配置参考文件生成失败，可稍后运行: ${INSTALL_PATH} --write-config-reference"
    fi
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
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
ExecStart=/usr/local/bin/VasmaX -c /etc/vasmax/config.yaml
Restart=on-failure
RestartSec=10
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
# VasmaX 主配置文件，可由 vasmax 菜单自动更新。
# 完整字段说明见同目录 config.reference.yaml；请勿在此文件保存重要手写注释。
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
subscription:
  dns_mode: auto
  test_url: https://www.gstatic.com/generate_204
nginx:
  long_connection_timeout: 86400s
sync:
  empty_users_apply_threshold: 3
  min_pull_interval_seconds: 30
  min_push_interval_seconds: 30
connection:
  keepalive_mode: auto
  keepalive_idle_seconds: 8
  keepalive_interval_seconds: 8
  keepalive_probes: 3
  websocket_heartbeat_seconds: 8
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
        echo "1) 保留当前配置（推荐。新版本可能会自动补全默认值）"
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

cleanup_vasmax_nginx_configs() {
    local conf_dir="/etc/nginx/conf.d"
    [[ -d "${conf_dir}" ]] || return 0

    local removed=0
    shopt -s nullglob
    for conf in "${conf_dir}"/*.conf; do
        if grep -qE "Managed by VasmaX|VasmaX domain|/etc/vasmax/subscribe|/vlessws|/vmessws|/vmesshup|/vlessgrpc|/trojangrpc" "${conf}" 2>/dev/null; then
            rm -f -- "${conf}"
            removed=$((removed + 1))
        fi
    done
    shopt -u nullglob

    if [[ "${removed}" -gt 0 ]]; then
        yellow "已清理 ${removed} 个 VasmaX Nginx 配置文件"
        if command -v nginx &>/dev/null; then
            nginx -t &>/dev/null && systemctl reload nginx 2>/dev/null || true
        fi
    fi
}

cleanup_vasmax_iptables_rules() {
    command -v iptables &>/dev/null || return 0

    # 只删除 VasmaX 端口跳跃常用规则，绝不 flush 整条 PREROUTING 链。
    while true; do
        local rule
        rule="$(iptables -t nat -S PREROUTING 2>/dev/null | grep -- "--dport 30000:40000" | grep -- "-j REDIRECT" | head -n1 || true)"
        [[ -n "${rule}" ]] || break
        # 将 "-A PREROUTING ..." 转成可删除参数。
        rule="${rule#-A PREROUTING }"
        # shellcheck disable=SC2086
        iptables -t nat -D PREROUTING ${rule} 2>/dev/null || break
    done
}

# --- 卸载 ---
uninstall() {
    yellow "开始卸载 VasmaX..."

    # 检测系统类型（卸载 Nginx 需要）
    detect_os

    # 停止 VasmaX 服务
    systemctl stop VasmaX 2>/dev/null || true
    systemctl disable VasmaX 2>/dev/null || true
    systemctl reset-failed VasmaX 2>/dev/null || true

    # 停止并清理 Xray-core
    systemctl stop xray.service 2>/dev/null || true
    systemctl disable xray.service 2>/dev/null || true
    systemctl reset-failed xray.service 2>/dev/null || true
    rm -f /etc/systemd/system/xray.service
    rm -rf /usr/local/xray-core/

    # 停止并清理 sing-box
    systemctl stop sing-box.service 2>/dev/null || true
    systemctl disable sing-box.service 2>/dev/null || true
    systemctl reset-failed sing-box.service 2>/dev/null || true
    rm -f /etc/systemd/system/sing-box.service
    rm -rf /usr/local/sing-box/

    # 删除 VasmaX 文件
    rm -f "${INSTALL_PATH}"
    rm -f "${SERVICE_FILE}"
    rm -f /usr/local/bin/vasmax
    rm -f "${INSTALLER_PATH}"

    # 清理 BBR/sysctl 配置文件
    rm -f /etc/sysctl.d/99-vasmax-bbr.conf
    rm -f /etc/sysctl.d/99-vasmax-optimize.conf
    rm -f /etc/sysctl.d/99-vasmax-ipv6.conf
    sysctl --system &>/dev/null || true

    # 检测是否安装了非原装内核
    local current_kernel
    current_kernel="$(uname -r)"
    if echo "${current_kernel}" | grep -qiE "bbrplus|xanmod|zen|lotserver|liquorix"; then
        yellow "检测到非原装内核: ${current_kernel}"
        yellow "该内核可能是通过 VasmaX BBR 菜单安装的"
        yellow "为安全起见，不会自动卸载内核（卸载运行中的内核会导致系统崩溃）"
        yellow "如需恢复原装内核，请手动操作："
        yellow "  1. 安装原装内核: apt install linux-image-amd64"
        yellow "  2. 重启后删除第三方内核: dpkg --purge <内核包名>"
    else
        green "当前内核为原装内核（${current_kernel}），BBR 配置已清理"
    fi

    # 清理 VasmaX 生成的 Nginx/iptables 配置。不会删除其他站点或其他 NAT 规则。
    cleanup_vasmax_iptables_rules
    cleanup_vasmax_nginx_configs

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

    # 删除安装脚本自身
    rm -f /root/install.sh 2>/dev/null || true

    systemctl daemon-reload
    green "VasmaX 已卸载（含 Xray-core、sing-box、BBR 配置）"
    yellow "注意: acme.sh 证书配置未被移除"
    yellow "如需清理请手动执行: ~/.acme.sh/acme.sh --uninstall"
}

# --- 主菜单 ---
show_menu() {
    echo ""
    green "VasmaX 部署脚本 v${SCRIPT_VERSION}"
    echo "─────────────────────────────"
    echo " 1. 安装/修复 VasmaX"
    echo " 2. 更新 VasmaX"
    echo " 3. 卸载 VasmaX"
    echo " 0. 退出"
    echo ""
    yellow "安装完成后的协议、证书、启动/停止、日志等操作请运行: vasmax"
    echo ""
    read -rp "请选择 [0-3]: " choice

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
        0) exit 0 ;;
        *) red "无效选择" ;;
    esac
}

do_install() {
    detect_os
    install_deps
    install_nginx
    init_dirs
    install_self_script
    download_binary
    write_config_reference
    setup_service
    if [[ "${OS_TYPE}" != "alpine" ]]; then
        systemctl start VasmaX
    fi
    green "安装完成！运行 vasmax 打开管理菜单"
    ${INSTALL_PATH} --version 2>/dev/null || true
}

do_update() {
    detect_os
    install_self_script
    local was_active=0
    local backup_file=""
    if [[ "${OS_TYPE}" != "alpine" ]] && systemctl is-active --quiet VasmaX 2>/dev/null; then
        was_active=1
    fi
    if [[ -f "${INSTALL_PATH}" ]]; then
        backup_file="${INSTALL_PATH}.bak.$(date +%Y%m%d%H%M%S)"
        cp -a "${INSTALL_PATH}" "${backup_file}"
        green "已备份当前二进制: ${backup_file}"
    fi

    update_failed() {
        local rc=$?
        trap - ERR
        red "更新失败，正在尝试回滚"
        if [[ -n "${backup_file}" && -f "${backup_file}" ]]; then
            cp -a "${backup_file}" "${INSTALL_PATH}"
            chmod 755 "${INSTALL_PATH}"
            yellow "已恢复旧二进制"
        fi
        if [[ "${OS_TYPE}" != "alpine" && "${was_active}" -eq 1 ]]; then
            systemctl start VasmaX 2>/dev/null || true
        fi
        exit "${rc}"
    }
    trap update_failed ERR

    if [[ "${OS_TYPE}" != "alpine" ]]; then
        systemctl stop VasmaX 2>/dev/null || true
    fi
    download_binary
    write_config_reference
    "${INSTALL_PATH}" --version >/dev/null
    if [[ "${OS_TYPE}" != "alpine" ]]; then
        systemctl start VasmaX
    fi
    trap - ERR
    green "更新完成"
    ${INSTALL_PATH} --version 2>/dev/null || true
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
        *) show_menu ;;
    esac
}

main "$@"
