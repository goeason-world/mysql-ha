#!/bin/bash
#
# MySQL HA 集群清理脚本
# 用于清理节点上已安装的 MySQL、etcd 和 HA Agent
# 使用方法: sudo ./clean.sh [选项]
#
# 选项:
#   --all       清理所有组件 (默认)
#   --mysql     只清理 MySQL
#   --etcd      只清理 etcd
#   --agent     只清理 HA Agent
#   --keep-data 保留数据目录
#   -y          跳过确认提示
#

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 默认配置
INSTALL_PATH="/opt/mysql-ha"
DATA_PATH="/data/mysqldata"
ETCD_DATA_PATH="/var/lib/etcd"

# 选项
CLEAN_MYSQL=false
CLEAN_ETCD=false
CLEAN_AGENT=false
KEEP_DATA=false
SKIP_CONFIRM=false

# 解析参数
if [ $# -eq 0 ]; then
    CLEAN_MYSQL=true
    CLEAN_ETCD=true
    CLEAN_AGENT=true
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            CLEAN_MYSQL=true
            CLEAN_ETCD=true
            CLEAN_AGENT=true
            shift
            ;;
        --mysql)
            CLEAN_MYSQL=true
            shift
            ;;
        --etcd)
            CLEAN_ETCD=true
            shift
            ;;
        --agent)
            CLEAN_AGENT=true
            shift
            ;;
        --keep-data)
            KEEP_DATA=true
            shift
            ;;
        -y|--yes)
            SKIP_CONFIRM=true
            shift
            ;;
        --install-path)
            INSTALL_PATH="$2"
            shift 2
            ;;
        --data-path)
            DATA_PATH="$2"
            shift 2
            ;;
        -h|--help)
            echo "MySQL HA 集群清理脚本"
            echo ""
            echo "用法: sudo $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --all           清理所有组件 (默认)"
            echo "  --mysql         只清理 MySQL"
            echo "  --etcd          只清理 etcd"
            echo "  --agent         只清理 HA Agent"
            echo "  --keep-data     保留数据目录"
            echo "  -y, --yes       跳过确认提示"
            echo "  --install-path  安装路径 (默认: /opt/mysql-ha)"
            echo "  --data-path     数据路径 (默认: /data/mysqldata)"
            echo "  -h, --help      显示帮助信息"
            exit 0
            ;;
        *)
            echo -e "${RED}未知选项: $1${NC}"
            exit 1
            ;;
    esac
done

# 检查 root 权限
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}请使用 root 权限运行此脚本${NC}"
    echo "用法: sudo $0"
    exit 1
fi

# 显示将要清理的内容
echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}MySQL HA 集群清理脚本${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo "将要清理的组件:"
$CLEAN_MYSQL && echo "  - MySQL"
$CLEAN_ETCD && echo "  - etcd"
$CLEAN_AGENT && echo "  - HA Agent (mypatroni)"
echo ""
echo "配置:"
echo "  安装路径: $INSTALL_PATH"
echo "  数据路径: $DATA_PATH"
echo "  保留数据: $KEEP_DATA"
echo ""

# 确认
if [ "$SKIP_CONFIRM" = false ]; then
    echo -e "${RED}警告: 此操作将删除所有相关数据和配置！${NC}"
    read -p "确定要继续吗? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "已取消"
        exit 0
    fi
fi

echo ""
echo -e "${GREEN}开始清理...${NC}"

# 清理 HA Agent
if [ "$CLEAN_AGENT" = true ]; then
    echo ""
    echo -e "${YELLOW}[1/3] 清理 HA Agent (mypatroni)...${NC}"
    
    # 停止服务
    if systemctl is-active --quiet mypatroni 2>/dev/null; then
        echo "  停止 mypatroni 服务..."
        systemctl stop mypatroni || true
    fi
    
    # 禁用服务
    if systemctl is-enabled --quiet mypatroni 2>/dev/null; then
        echo "  禁用 mypatroni 服务..."
        systemctl disable mypatroni || true
    fi
    
    # 删除服务文件
    if [ -f /etc/systemd/system/mypatroni.service ]; then
        echo "  删除服务文件..."
        rm -f /etc/systemd/system/mypatroni.service
    fi
    
    # 删除配置
    if [ -d /etc/mypatroni ]; then
        echo "  删除配置目录..."
        rm -rf /etc/mypatroni
    fi
    
    # 删除日志
    if [ -d /var/log/mypatroni ]; then
        echo "  删除日志目录..."
        rm -rf /var/log/mypatroni
    fi
    
    # 删除安装目录
    if [ -d "$INSTALL_PATH/mypatroni" ]; then
        echo "  删除安装目录..."
        rm -rf "$INSTALL_PATH/mypatroni"
    fi
    
    echo -e "${GREEN}  HA Agent 清理完成${NC}"
fi

# 清理 MySQL
if [ "$CLEAN_MYSQL" = true ]; then
    echo ""
    echo -e "${YELLOW}[2/3] 清理 MySQL...${NC}"
    
    # 停止服务
    if systemctl is-active --quiet mysqld 2>/dev/null; then
        echo "  停止 mysqld 服务..."
        systemctl stop mysqld || true
    fi
    if systemctl is-active --quiet mysql 2>/dev/null; then
        echo "  停止 mysql 服务..."
        systemctl stop mysql || true
    fi
    
    # 禁用服务
    if systemctl is-enabled --quiet mysqld 2>/dev/null; then
        echo "  禁用 mysqld 服务..."
        systemctl disable mysqld || true
    fi
    if systemctl is-enabled --quiet mysql 2>/dev/null; then
        echo "  禁用 mysql 服务..."
        systemctl disable mysql || true
    fi
    
    # 杀死残留进程
    echo "  杀死残留 MySQL 进程..."
    pkill -9 mysqld 2>/dev/null || true
    pkill -9 mysqld_safe 2>/dev/null || true
    
    # 删除服务文件
    if [ -f /etc/systemd/system/mysqld.service ]; then
        echo "  删除服务文件..."
        rm -f /etc/systemd/system/mysqld.service
    fi
    if [ -f /etc/systemd/system/mysql.service ]; then
        rm -f /etc/systemd/system/mysql.service
    fi
    
    # 删除配置
    if [ -f /etc/my.cnf ]; then
        echo "  删除配置文件..."
        rm -f /etc/my.cnf
    fi
    if [ -d /etc/my.cnf.d ]; then
        rm -rf /etc/my.cnf.d
    fi
    
    # 删除运行时目录
    echo "  删除运行时目录..."
    rm -rf /var/run/mysqld
    rm -rf /var/log/mysql
    rm -f /tmp/mysql.sock
    rm -f /tmp/mysql.sock.lock
    
    # 删除安装目录
    if [ -d "$INSTALL_PATH/mysql" ]; then
        echo "  删除安装目录..."
        rm -rf "$INSTALL_PATH/mysql"
    fi
    
    # 删除数据目录
    if [ "$KEEP_DATA" = false ]; then
        if [ -d "$DATA_PATH" ]; then
            echo "  删除数据目录..."
            rm -rf "$DATA_PATH"
        fi
        # 也清理默认数据目录
        if [ -d /var/lib/mysql ]; then
            rm -rf /var/lib/mysql
        fi
    else
        echo "  保留数据目录: $DATA_PATH"
    fi
    
    echo -e "${GREEN}  MySQL 清理完成${NC}"
fi

# 清理 etcd
if [ "$CLEAN_ETCD" = true ]; then
    echo ""
    echo -e "${YELLOW}[3/3] 清理 etcd...${NC}"
    
    # 停止服务
    if systemctl is-active --quiet etcd 2>/dev/null; then
        echo "  停止 etcd 服务..."
        systemctl stop etcd || true
    fi
    
    # 禁用服务
    if systemctl is-enabled --quiet etcd 2>/dev/null; then
        echo "  禁用 etcd 服务..."
        systemctl disable etcd || true
    fi
    
    # 杀死残留进程
    echo "  杀死残留 etcd 进程..."
    pkill -9 etcd 2>/dev/null || true
    
    # 删除服务文件
    if [ -f /etc/systemd/system/etcd.service ]; then
        echo "  删除服务文件..."
        rm -f /etc/systemd/system/etcd.service
    fi
    
    # 删除符号链接
    rm -f /usr/local/bin/etcd 2>/dev/null || true
    rm -f /usr/local/bin/etcdctl 2>/dev/null || true
    
    # 删除安装目录
    if [ -d "$INSTALL_PATH/etcd" ]; then
        echo "  删除安装目录..."
        rm -rf "$INSTALL_PATH/etcd"
    fi
    
    # 删除数据目录
    if [ "$KEEP_DATA" = false ]; then
        if [ -d "$ETCD_DATA_PATH" ]; then
            echo "  删除数据目录..."
            rm -rf "$ETCD_DATA_PATH"
        fi
    else
        echo "  保留数据目录: $ETCD_DATA_PATH"
    fi
    
    echo -e "${GREEN}  etcd 清理完成${NC}"
fi

# 重新加载 systemd
echo ""
echo "重新加载 systemd..."
systemctl daemon-reload

# 清理空的安装目录
if [ -d "$INSTALL_PATH" ]; then
    if [ -z "$(ls -A $INSTALL_PATH 2>/dev/null)" ]; then
        echo "删除空的安装目录..."
        rmdir "$INSTALL_PATH" 2>/dev/null || true
    fi
fi

# 清理临时文件
echo "清理临时文件..."
rm -f /tmp/etcd.tar.gz 2>/dev/null || true
rm -rf /tmp/etcd-v*-linux-amd64 2>/dev/null || true
rm -f /tmp/mysql.tar.gz /tmp/mysql.tar.xz 2>/dev/null || true
rm -rf /tmp/mysql-*-linux-* 2>/dev/null || true

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}清理完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "节点现在是干净的，可以重新安装集群。"
echo ""
