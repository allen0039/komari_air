#!/bin/sh
# Komari / komari_air Agent uninstaller. Linux only; no network requests.
set -eu

service_name=komari-agent
install_dir=
explicit_dir=false
user_mode=false
dry_run=false
config_file=
managed_file=
service_kind=none
service_file=
systemd_dir=/etc/systemd/system
initd_dir=/etc/init.d
upstart_dir=/etc/init
proc_dir=/proc
default_dir=/opt/komari
log_dir=/var/log/komari

info() { printf '%s\n' "$*"; }
die() { printf '错误：%s\n' "$*" >&2; exit 1; }
usage() {
    cat <<'EOF'
Komari Agent 一键卸载（Linux）

  sudo sh uninstall-agent.sh
  sudo sh uninstall-agent.sh --dry-run
  sudo sh uninstall-agent.sh --service-name my-agent --install-dir /srv/komari
  sh uninstall-agent.sh --user

选项：
  --service-name NAME       服务名；默认 komari-agent
  --install-service-name   同 --service-name，兼容安装脚本参数
  --install-dir PATH       安装目录；默认从服务识别，否则 /opt/komari
  --user                   当前用户的 systemd 服务；以安装用户运行，不加 sudo
  --config-file PATH       同时删除自行指定的 JSON 启动配置及其 .tmp/.bak
  --managed-config PATH    同时删除外置 managed-config 及其 .tmp/.bak
  --dry-run                只预览；不会停止服务、终止进程或删除文件
  -h, --help               显示说明

清理程序、自动发现身份、运行配置、流量统计、升级副本和服务定义。
只删除已知 Agent 文件；目录中其他文件会保留，空安装目录会删除。
仅处理选定服务，不修改面板节点记录，不清空系统 journal。
Docker、Windows、macOS、NixOS 不由本脚本处理。
EOF
}

need_value() { [ "$#" -ge 2 ] && [ -n "$2" ] || die "$1 缺少参数"; }
while [ "$#" -gt 0 ]; do
    case "$1" in
        --service-name|--install-service-name) need_value "$@"; service_name=$2; shift 2 ;;
        --install-dir) need_value "$@"; install_dir=$2; explicit_dir=true; shift 2 ;;
        --config-file) need_value "$@"; config_file=$2; shift 2 ;;
        --managed-config) need_value "$@"; managed_file=$2; shift 2 ;;
        --user) user_mode=true; shift ;;
        --dry-run) dry_run=true; shift ;;
        -h|--help) usage; exit 0 ;;
        *) die "未知参数：$1（运行 --help 查看说明）" ;;
    esac
done

case "$service_name" in
    ''|[!a-zA-Z0-9_]*|*[!a-zA-Z0-9_.@-]*) die "服务名格式无效" ;;
esac
[ "$(uname -s)" = Linux ] || die "此脚本适用于 Linux"
[ ! -f /etc/NIXOS ] || die "NixOS 请先从声明式配置删除 Agent 服务并运行 nixos-rebuild switch"
if [ "$user_mode" = true ]; then
    [ "$(id -u)" -ne 0 ] || die "--user 请以安装 Agent 的普通用户运行，不加 sudo"
    systemd_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
    default_dir="${XDG_DATA_HOME:-$HOME/.local/share}/komari"
elif [ "$(id -u)" -ne 0 ] && [ "$dry_run" = false ]; then
    die "系统服务卸载请使用 sudo sh uninstall-agent.sh；用户服务请加 --user"
fi

ctl() {
    if [ "$user_mode" = true ]; then systemctl --user "$@"; else systemctl "$@"; fi
}
run() {
    if [ "$dry_run" = true ]; then
        printf '[预览]'; for arg do printf ' <%s>' "$arg"; done; printf '\n'
    else
        "$@" || die "操作失败：$1"
    fi
}
remove_file() {
    [ -e "$1" ] || [ -L "$1" ] || return 0
    [ ! -d "$1" ] || [ -L "$1" ] || die "预期为文件却发现目录，保留：$1"
    run rm -f -- "$1"
}

# Canonicalize existing parents, without following the final file itself.
# No sourcing/eval of service files: they can contain arbitrary shell code.
check_dir_safety() {
    case "$1" in
        /|/etc|/etc/*|/bin|/bin/*|/sbin|/sbin/*|/lib|/lib/*|/lib64|/lib64/*|\
        /usr|/usr/bin|/usr/bin/*|/usr/sbin|/usr/sbin/*|/usr/lib|/usr/lib/*|\
        /usr/local|/usr/local/bin|/usr/local/bin/*|/usr/local/sbin|/usr/local/sbin/*|\
        /dev|/dev/*|/proc|/proc/*|/sys|/sys/*|/var|/var/log|/var/lib|/var/run|\
        /opt|/srv|/home|/root|/tmp|/run|/run/*) die "拒绝清理系统目录：$1" ;;
    esac
    [ "$1" != "${HOME%/}" ] || die "拒绝清理用户主目录"
}
normalize_dir() {
    case "$1" in /*) ;; *) die "目录必须为绝对路径：$1" ;; esac
    case "$1" in *'
'*|*'/../'*|*/..|*'/./'*|*/.) die "目录含有不允许的路径片段：$1" ;; esac
    normalized=${1%/}
    [ -n "$normalized" ] || die "拒绝使用根目录"
    check_dir_safety "$normalized"
    if [ -d "$normalized" ]; then
        normalized=$(CDPATH= cd -- "$normalized" && pwd -P) || die "无法读取目录：$1"
    else
        parent=${normalized%/*}
        [ -n "$parent" ] || parent=/
        if [ -d "$parent" ]; then
            parent=$(CDPATH= cd -- "$parent" && pwd -P) || die "无法读取父目录"
            normalized="${parent%/}/${normalized##*/}"
        fi
    fi
    check_dir_safety "$normalized"
}
normalize_extra_file() {
    case "$1" in /*) ;; *) die "配置文件必须为绝对路径：$1" ;; esac
    leaf=${1##*/}
    [ -n "$leaf" ] || die "配置文件路径不能以 / 结尾"
    case "$leaf" in *.json) ;; *) die "额外配置只接受 .json 文件：$1" ;; esac
    normalize_dir "${1%/*}"
    extra_path="$normalized/$leaf"
    [ ! -d "$extra_path" ] || [ -L "$extra_path" ] || die "配置路径是目录：$extra_path"
}

# Discover only the selected service, so other Agent instances stay untouched.
if command -v systemctl >/dev/null 2>&1; then
    fragment=$(ctl show "$service_name.service" -p FragmentPath --value 2>/dev/null || true)
    if [ -n "$fragment" ] && [ -f "$fragment" ]; then
        service_file=$fragment; service_kind=systemd
    elif [ -f "$systemd_dir/$service_name.service" ]; then
        service_file="$systemd_dir/$service_name.service"; service_kind=systemd
    fi
fi
if [ "$service_kind" = none ] && [ "$user_mode" = false ]; then
    if [ -f "$initd_dir/$service_name" ]; then
        service_file="$initd_dir/$service_name"
        if command -v rc-service >/dev/null 2>&1; then service_kind=openrc
        elif [ -f /etc/rc.common ]; then service_kind=procd
        else die "发现 init.d 服务但不能识别管理方式"; fi
    elif [ -f "$upstart_dir/$service_name.conf" ]; then
        service_file="$upstart_dir/$service_name.conf"; service_kind=upstart
    fi
fi

detected_dir=
if [ -n "$service_file" ]; then
    if [ "$service_kind" = procd ]; then
        # install.sh's procd template has no description/banner.
        grep -Eq '^USE_PROCD=1[[:space:]]*$' "$service_file" &&
            grep -q 'procd_open_instance' "$service_file" ||
            die "服务不符合 Agent 的 procd 模板，保留：$service_file"
    else
        grep -Eq 'Komari Agent|Komari monitoring agent|KOMARI Agent' "$service_file" ||
            die "服务不具备 Komari Agent 标识，保留：$service_file"
    fi
    case "$service_kind" in
        systemd)
            detected_dir=$(ctl show "$service_name.service" -p WorkingDirectory --value 2>/dev/null || true)
            if [ -z "$detected_dir" ]; then
                detected_dir=$(sed -n 's/^WorkingDirectory=//p' "$service_file" | tail -n 1)
            fi ;;
        openrc) detected_dir=$(sed -n 's/^directory="\(.*\)"$/\1/p' "$service_file" | tail -n 1) ;;
        procd)
            program=$(sed -n 's/^PROG="\(.*\)"$/\1/p' "$service_file" | tail -n 1)
            [ -z "$program" ] || detected_dir=${program%/*} ;;
        upstart) detected_dir=$(sed -n 's/^chdir //p' "$service_file" | tail -n 1) ;;
    esac
fi
if [ -n "$detected_dir" ]; then
    normalize_dir "$detected_dir"; detected_dir=$normalized
    if [ "$explicit_dir" = true ]; then
        normalize_dir "$install_dir"
        [ "$normalized" = "$detected_dir" ] || die "指定目录与服务目录不同；服务目录为 $detected_dir"
    else install_dir=$detected_dir; fi
fi
[ -n "$install_dir" ] || install_dir=$default_dir
normalize_dir "$install_dir"; install_dir=$normalized
if [ -n "$service_file" ]; then
    # A description alone must not authorize stopping an unrelated executable.
    grep -Fq "$install_dir/agent" "$service_file" ||
        grep -Fq "$install_dir/komari-agent" "$service_file" ||
        die "服务程序与安装目录不匹配，保留：$service_file"
fi
[ -z "$config_file" ] || { normalize_extra_file "$config_file"; config_file=$extra_path; }
[ -z "$managed_file" ] || { normalize_extra_file "$managed_file"; managed_file=$extra_path; }

info "服务：${service_name}（${service_kind}）；安装目录：${install_dir}"
if [ "$dry_run" = true ]; then info '当前为预览模式，不执行卸载。'; fi

# Remove restart supervision before touching the executable or persistent state.
case "$service_kind" in
    systemd)
        run ctl stop "$service_name.service"
        run ctl disable "$service_name.service"
        remove_file "$service_file"
        if [ "$service_file" != "$systemd_dir/$service_name.service" ]; then
            remove_file "$systemd_dir/$service_name.service"
        fi
        if [ -e "$systemd_dir/$service_name.service.d" ] || [ -L "$systemd_dir/$service_name.service.d" ]; then
            run rm -rf -- "$systemd_dir/$service_name.service.d"
        fi
        run ctl daemon-reload ;;
    openrc)
        run rc-service "$service_name" stop
        run rc-update --all delete "$service_name"
        remove_file "$service_file" ;;
    procd)
        run "$service_file" stop
        run "$service_file" disable
        remove_file "$service_file" ;;
    upstart)
        if initctl status "$service_name" 2>/dev/null | grep -q 'start/running'; then
            run initctl stop "$service_name"
        fi
        remove_file "$service_file"
        run initctl reload-configuration ;;
    none)
        # Loaded systemd units without a backing file may still respawn.
        if command -v systemctl >/dev/null 2>&1 && ctl is-active --quiet "$service_name.service" 2>/dev/null; then
            die "服务仍活动但无法核实服务定义；保留文件，请先处理该服务"
        fi ;;
esac

agent_pids() {
    for process in "$proc_dir"/[0-9]*/exe; do
        [ -L "$process" ] || continue
        executable=$(readlink "$process" 2>/dev/null || true)
        executable=${executable% (deleted)}
        case "$executable" in
            "$install_dir/agent"|"$install_dir/komari-agent"|\
            "$install_dir/.agent.old"|"$install_dir/.komari-agent.old")
                process=${process%/exe}; printf '%s\n' "${process##*/}" ;;
        esac
    done
}
# Match executable paths exactly, never kill every process named "agent".
pids=$(agent_pids)
if [ -n "$pids" ]; then
    for pid in $pids; do
        if [ "$dry_run" = true ]; then run kill -TERM "$pid"
        else kill -TERM "$pid" 2>/dev/null || true; fi
    done
    if [ "$dry_run" = false ]; then
        remaining=10
        while [ -n "$(agent_pids)" ] && [ "$remaining" -gt 0 ]; do
            sleep 1; remaining=$((remaining - 1))
        done
        [ -z "$(agent_pids)" ] || die "Agent 进程仍在运行；保留程序和配置，请检查其他守护进程"
    fi
fi

for basename in agent komari-agent .agent.new .agent.old .komari-agent.new .komari-agent.old \
    agent.hardening.gz komari-agent.hardening.gz \
    auto-discovery.json managed-config.json managed-config.json.tmp managed-config.json.bak \
    net_static.json net_static.json.tmp net_static.json.bak; do
    remove_file "$install_dir/$basename"
done
for backup in "$install_dir"/agent.before-* "$install_dir"/komari-agent.before-* \
    "$install_dir"/agent.backup-* "$install_dir"/komari-agent.backup-* \
    "$install_dir"/agent.pre-* "$install_dir"/komari-agent.pre-*; do
    remove_file "$backup"
done
for extra in "$config_file" "$managed_file"; do
    [ -n "$extra" ] || continue
    remove_file "$extra"; remove_file "$extra.tmp"; remove_file "$extra.bak"
done

# Old deployment procedures also copied service definitions outside install_dir.
# They can contain credentials. Remove only backups referencing this Agent path.
cleanup_service_backups() {
for service_stem do
    for service_backup in "$service_stem".before-* "$service_stem".pre-* \
        "$service_stem".backup-* "$service_stem.bak"; do
        [ -e "$service_backup" ] || [ -L "$service_backup" ] || continue
        if [ -f "$service_backup" ] && { grep -Fq "$install_dir/agent" "$service_backup" ||
            grep -Fq "$install_dir/komari-agent" "$service_backup"; }; then
            remove_file "$service_backup"
        else
            info "保留未确认归属的服务备份：$service_backup"
        fi
    done
done
}
if [ "$user_mode" = true ]; then
    cleanup_service_backups "$systemd_dir/$service_name.service"
else
    cleanup_service_backups "$systemd_dir/$service_name.service" \
        "$initd_dir/$service_name" "$upstart_dir/$service_name.conf"
fi

# Older Linux deployments may have file logs. Preserve unidentified files.
if [ "$user_mode" = false ] && [ "$install_dir" = "$default_dir" ] && [ "$service_name" = komari-agent ]; then
    if [ -d "$log_dir" ] && [ ! -L "$log_dir" ]; then
        for logfile in "$log_dir"/agent.log "$log_dir"/agent.log.* \
            "$log_dir"/komari-agent.log "$log_dir"/komari-agent.log.*; do
            remove_file "$logfile"
        done
        if [ "$dry_run" = false ]; then rmdir -- "$log_dir" 2>/dev/null || true; fi
    fi
fi
if [ "$dry_run" = false ]; then
    if [ "$service_kind" = systemd ]; then
        ctl reset-failed "$service_name.service" >/dev/null 2>&1 || true
        ctl is-active --quiet "$service_name.service" 2>/dev/null && die "服务仍然活动"
    fi
    if [ -d "$install_dir" ]; then
        if ! rmdir -- "$install_dir" 2>/dev/null; then
            info "保留目录中的其他文件：${install_dir}（如有自行配置的文件，可用 --config-file 指定）"
        fi
    fi
    info 'Agent 卸载完成。面板中的节点记录需在后台另行删除。'
else
    info '预览完成。去掉 --dry-run 即可执行卸载。'
fi
