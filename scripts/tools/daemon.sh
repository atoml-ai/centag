#!/bin/bash
# Author: Centag
# 守护进程脚本
# 用法: ./daemon.sh [work_dir]
# 示例: ./daemon.sh
#       ./daemon.sh ./bin
#
# 环境变量：
#   DAEMON_DEBUG - 调试模式（true=前台运行输出到终端，false=后台运行，默认: false）
#   DAEMON_CHECK_INTERVAL - 检查间隔（秒，默认: 5）
#
# 注意：当 LLM_PROXY_LOG_OUTPUT 设置为 "file" 时，日志会同时输出到控制台和文件
#       无论是前台还是后台运行，都可以在两个地方看到日志

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 跟踪 tail -f 进程 PID，用于在退出时清理
TAIL_PID=""

# 信号处理
cleanup() {
    echo ""
    echo -e "${YELLOW}收到退出信号，正在停止守护进程...${NC}"
    
    # 设置退出标志，停止主循环
    DAEMON_EXIT=1
    
    # 停止日志跟踪进程
    if [ -n "$TAIL_PID" ] && kill -0 "$TAIL_PID" 2>/dev/null; then
        kill -TERM "$TAIL_PID" 2>/dev/null || true
        TAIL_PID=""
    fi

    # 停止服务进程
    if [ -n "$SERVICE_PID" ] && kill -0 "$SERVICE_PID" 2>/dev/null; then
        echo -e "${YELLOW}正在停止服务 (PID: $SERVICE_PID)...${NC}"
        kill -TERM "$SERVICE_PID" 2>/dev/null || true
        
        # 等待进程退出
        local count=0
        while [ $count -lt 50 ]; do
            if ! kill -0 "$SERVICE_PID" 2>/dev/null; then
                break
            fi
            sleep 0.1
            count=$((count + 1))
        done
        
        # 如果还在运行，强制停止
        if kill -0 "$SERVICE_PID" 2>/dev/null; then
            echo -e "${YELLOW}强制停止服务...${NC}"
            kill -KILL "$SERVICE_PID" 2>/dev/null || true
            sleep 0.5
        fi
    fi
    
    # 清理 PID 文件
    if [ -f "$PID_FILE" ]; then
        rm -f "$PID_FILE"
    fi
    
    # 清理守护进程 PID 文件
    local daemon_pid_file=""
    if [ -n "$WORK_DIR" ]; then
        daemon_pid_file="$WORK_DIR/storage/centag.daemon.pid"
        if [ -f "$daemon_pid_file" ]; then
            rm -f "$daemon_pid_file"
        fi
    fi
    
    echo -e "${GREEN}✅ 守护进程已停止${NC}"
    exit 0
}

trap cleanup SIGTERM SIGINT

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 解析参数
WORK_DIR="${1:-}"

# 如果没有指定工作目录，使用项目根目录
if [ -z "$WORK_DIR" ]; then
    WORK_DIR="$PROJECT_ROOT"
else
    # 转换为绝对路径
    if [ ! -d "$WORK_DIR" ]; then
        echo -e "${RED}错误：工作目录不存在: $WORK_DIR${NC}"
        exit 1
    fi
    WORK_DIR="$(cd "$WORK_DIR" && pwd)"
fi

# 服务配置
SERVICE_NAME="centag"
SERVICE_BIN="${LLM_PROXY_BINARY:-}"
PORT="${LLM_PROXY_SERVER_PORT:-${LLM_PROXY_PORT:-20060}}"
LOG_LEVEL="${LLM_PROXY_LOG_LEVEL:-info}"
CHECK_INTERVAL="${DAEMON_CHECK_INTERVAL:-5}"
LOG_FILE="${LLM_PROXY_LOG_FILE:-$WORK_DIR/logs/centag.log}"
PID_FILE="${LLM_PROXY_PID_FILE:-$WORK_DIR/storage/centag.pid}"
DAEMON_DEBUG="${DAEMON_DEBUG:-false}"

# 查找服务二进制文件
find_service_binary() {
    # 如果已指定，直接使用
    if [ -n "$SERVICE_BIN" ] && [ -f "$SERVICE_BIN" ] && [ -x "$SERVICE_BIN" ]; then
        echo "$SERVICE_BIN"
        return 0
    fi
    
    # 尝试从常见位置查找
    local current_dir=$(pwd)
    local install_root="${CENTAG_INSTALL_ROOT:-$HOME/.centag}"
    local edition="${CENTAG_EDITION:-personal}"
    local locations=(
        "$WORK_DIR/$SERVICE_NAME"
        "$WORK_DIR/centag-${edition}"
        "$WORK_DIR/bin/$SERVICE_NAME"
        "$install_root/lib/${edition}/centag-${edition}"
        "$install_root/bin/centag"
        "$PROJECT_ROOT/$SERVICE_NAME"
        "$PROJECT_ROOT/var/bin/$SERVICE_NAME"
        "$PROJECT_ROOT/bin/server/$SERVICE_NAME"
        "$current_dir/$SERVICE_NAME"
        "$current_dir/bin/$SERVICE_NAME"
        "/usr/local/bin/$SERVICE_NAME"
        "/app/$SERVICE_NAME"
        "/app/bin/$SERVICE_NAME"
        "/app/bin/centag-personal"
        "/app/bin/centag-minimal"
    )
    
    for location in "${locations[@]}"; do
        if [ -f "$location" ] && [ -x "$location" ]; then
            # 转换为绝对路径
            local abs_path
            if abs_path=$(cd "$(dirname "$location")" && pwd)/$(basename "$location") 2>/dev/null; then
                if [ -f "$abs_path" ] && [ -x "$abs_path" ]; then
                    echo "$abs_path"
                    return 0
                fi
            fi
            echo "$location"
            return 0
        fi
    done
    
    return 1
}

# 通过端口查找并杀掉占用端口的进程
kill_port_processes() {
    local port=$1
    local found_pids=""
    
    if [ -z "$port" ]; then
        return 1
    fi
    
    # 使用 lsof（最常用）
    if command -v lsof >/dev/null 2>&1; then
        found_pids=$(lsof -ti ":$port" 2>/dev/null || true)
        if [ -n "$found_pids" ]; then
            echo -e "${YELLOW}⚠️  警告：端口 $port 已被占用${NC}"
            echo -e "${YELLOW}   正在清理进程...${NC}"
            
            echo "$found_pids" | while IFS= read -r pid; do
                if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                    kill -TERM "$pid" 2>/dev/null || true
                fi
            done
            
            sleep 2
            
            found_pids=$(lsof -ti ":$port" 2>/dev/null || true)
            if [ -n "$found_pids" ]; then
                echo -e "${YELLOW}   强制终止进程...${NC}"
                echo "$found_pids" | while IFS= read -r pid; do
                    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                        kill -KILL "$pid" 2>/dev/null || true
                    fi
                done
                sleep 1
            fi
        fi
    fi
    
    # 使用 netstat（备用方法）
    if [ -z "$found_pids" ] && command -v netstat >/dev/null 2>&1; then
        found_pids=$(netstat -tlnp 2>/dev/null | grep ":$port " | awk '{print $7}' | cut -d'/' -f1 | grep -E '^[0-9]+$' | sort -u || true)
        if [ -n "$found_pids" ]; then
            echo "$found_pids" | while IFS= read -r pid; do
                if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                    kill -TERM "$pid" 2>/dev/null || true
                fi
            done
            sleep 2
            
            found_pids=$(netstat -tlnp 2>/dev/null | grep ":$port " | awk '{print $7}' | cut -d'/' -f1 | grep -E '^[0-9]+$' | sort -u || true)
            if [ -n "$found_pids" ]; then
                echo "$found_pids" | while IFS= read -r pid; do
                    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                        kill -KILL "$pid" 2>/dev/null || true
                    fi
                done
            fi
        fi
    fi
    
    # 使用 ss（备用方法）
    if [ -z "$found_pids" ] && command -v ss >/dev/null 2>&1; then
        found_pids=$(ss -tlnp 2>/dev/null | grep ":$port " | grep -oP 'pid=\K[0-9]+' | sort -u || true)
        if [ -n "$found_pids" ]; then
            echo "$found_pids" | while IFS= read -r pid; do
                if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                    kill -TERM "$pid" 2>/dev/null || true
                fi
            done
            sleep 2
            
            found_pids=$(ss -tlnp 2>/dev/null | grep ":$port " | grep -oP 'pid=\K[0-9]+' | sort -u || true)
            if [ -n "$found_pids" ]; then
                echo "$found_pids" | while IFS= read -r pid; do
                    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                        kill -KILL "$pid" 2>/dev/null || true
                    fi
                done
            fi
        fi
    fi
    
    return 0
}

# 启动服务
start_service() {
    local service_bin="$1"
    
    cd "$WORK_DIR" || {
        echo -e "${RED}❌ 错误：无法切换到工作目录: $WORK_DIR${NC}"
        return 1
    }
    
    # 创建必要的目录
    mkdir -p "$(dirname "$LOG_FILE")" "$(dirname "$PID_FILE")"
    
    # 将二进制文件路径转换为相对于当前工作目录的路径
    local service_bin_final=""
    
    if [ -f "./$SERVICE_NAME" ] && [ -x "./$SERVICE_NAME" ]; then
        service_bin_final="./$SERVICE_NAME"
    elif [ -f "./bin/$SERVICE_NAME" ] && [ -x "./bin/$SERVICE_NAME" ]; then
        service_bin_final="./bin/$SERVICE_NAME"
    elif [[ "$service_bin" == "$WORK_DIR"/* ]]; then
        service_bin_final="./$(basename "$service_bin")"
    elif [[ "$service_bin" == /* ]] && [ -f "$service_bin" ] && [ -x "$service_bin" ]; then
        service_bin_final="$service_bin"
    elif [[ "$service_bin" == *"/"* ]] && [ -f "./$(basename "$service_bin")" ] && [ -x "./$(basename "$service_bin")" ]; then
        service_bin_final="./$(basename "$service_bin")"
    elif [ -f "./$service_bin" ] && [ -x "./$service_bin" ]; then
        service_bin_final="./$service_bin"
    elif [ -f "$service_bin" ] && [ -x "$service_bin" ]; then
        service_bin_final="$service_bin"
    fi
    
    if [ -z "$service_bin_final" ] || [ ! -f "$service_bin_final" ] || [ ! -x "$service_bin_final" ]; then
        echo -e "${RED}❌ 错误：$SERVICE_NAME 二进制文件不存在: $service_bin (工作目录: $WORK_DIR)${NC}"
        return 1
    fi
    
    service_bin="$service_bin_final"
    
    echo -e "${BLUE}🚀 启动 $SERVICE_NAME...${NC}"
    echo -e "${BLUE}   二进制文件: $service_bin${NC}"
    echo -e "${BLUE}   工作目录: $WORK_DIR${NC}"
    echo -e "${BLUE}   端口: $PORT${NC}"
    
    # 检查并清理端口占用
    kill_port_processes "$PORT"
    
    if [ "$DAEMON_DEBUG" = "true" ]; then
        echo -e "${BLUE}   Debug 模式：前台运行，输出到终端${NC}"
        # Debug 模式前台运行
        if command -v stdbuf >/dev/null 2>&1; then
            exec stdbuf -oL -eL "$service_bin"
        else
            exec "$service_bin"
        fi
    else
        echo -e "${BLUE}   日志文件: $LOG_FILE${NC}"
        echo -e "${BLUE}   日志同时输出到控制台和文件（由 LLM_PROXY_LOG_OUTPUT 环境变量控制）${NC}"
        nohup "$service_bin" >> "$LOG_FILE" 2>&1 &
        local pid=$!
        
        # 保存 PID
        echo "$pid" > "$PID_FILE"
        SERVICE_PID=$pid
        
        # 等待服务启动
        local max_wait=10
        local waited=0
        local service_ready=0
        
        while [ $waited -lt $max_wait ]; do
            if ! kill -0 "$pid" 2>/dev/null; then
                break
            fi
            
            # 检查端口是否在监听
            local port_listening=0
            if command -v lsof >/dev/null 2>&1; then
                if lsof -i ":$PORT" >/dev/null 2>&1; then
                    port_listening=1
                fi
            elif command -v netstat >/dev/null 2>&1; then
                if netstat -tln 2>/dev/null | grep -q ":$PORT "; then
                    port_listening=1
                fi
            elif command -v ss >/dev/null 2>&1; then
                if ss -tln 2>/dev/null | grep -q ":$PORT "; then
                    port_listening=1
                fi
            else
                port_listening=1
            fi
            
            if [ $port_listening -eq 1 ]; then
                service_ready=1
                break
            fi
            
            sleep 1
            waited=$((waited + 1))
        done
        
        if [ $service_ready -eq 1 ] && kill -0 "$pid" 2>/dev/null; then
            echo -e "${GREEN}✅ $SERVICE_NAME 已启动 (PID: $pid, 端口: $PORT)${NC}"
            # 在后台跟踪日志文件，将内容实时打印到控制台
            # 停掉旧的 tail 进程（服务重启时）
            if [ -n "$TAIL_PID" ] && kill -0 "$TAIL_PID" 2>/dev/null; then
                kill -TERM "$TAIL_PID" 2>/dev/null || true
                TAIL_PID=""
            fi
            tail -f "$LOG_FILE" &
            TAIL_PID=$!
            return 0
        else
            echo -e "${RED}❌ $SERVICE_NAME 启动失败${NC}"
            if ! kill -0 "$pid" 2>/dev/null; then
                echo -e "${YELLOW}   进程已退出${NC}"
            else
                echo -e "${YELLOW}   进程在运行但端口 $PORT 未监听${NC}"
            fi
            if [ -f "$LOG_FILE" ]; then
                echo -e "${YELLOW}最后几行日志:${NC}"
                tail -10 "$LOG_FILE" 2>/dev/null || true
            fi
            return 1
        fi
    fi
}

# 检查服务是否在运行
check_service() {
    if [ -z "$SERVICE_PID" ]; then
        return 1
    fi
    
    if ! kill -0 "$SERVICE_PID" 2>/dev/null; then
        return 1
    fi
    
    # 检查端口是否在监听
    local port_listening=0
    if command -v lsof >/dev/null 2>&1; then
        if lsof -i ":$PORT" >/dev/null 2>&1; then
            port_listening=1
        fi
    elif command -v netstat >/dev/null 2>&1; then
        if netstat -tln 2>/dev/null | grep -q ":$PORT "; then
            port_listening=1
        fi
    elif command -v ss >/dev/null 2>&1; then
        if ss -tln 2>/dev/null | grep -q ":$PORT "; then
            port_listening=1
        fi
    else
        port_listening=1
    fi
    
    if [ $port_listening -eq 1 ]; then
        return 0
    else
        return 1
    fi
}

# 检查热更新标记
check_update_marker() {
    local update_stop_file="$WORK_DIR/storage/update_stop"
    
    if [ -f "$update_stop_file" ]; then
        echo -e "${YELLOW}⚠️  检测到热更新标记${NC}"
        
        # 读取更新标记信息
        if [ -f "$WORK_DIR/storage/update_marker" ]; then
            local marker_time=$(cat "$WORK_DIR/storage/update_marker" 2>/dev/null | grep -o '"timestamp"[^,]*' | cut -d'"' -f4 || echo "")
            if [ -n "$marker_time" ]; then
                echo -e "${YELLOW}   更新版本: $marker_time${NC}"
            fi
        fi
        
        # 停止服务进程
        if [ -n "$SERVICE_PID" ] && kill -0 "$SERVICE_PID" 2>/dev/null; then
            echo -e "${YELLOW}   停止服务 (PID: $SERVICE_PID)...${NC}"
            
            # 发送 SIGTERM 信号
            kill -TERM "$SERVICE_PID" 2>/dev/null || true
            
            # 等待进程退出
            local max_wait=10
            local waited=0
            while [ $waited -lt $max_wait ]; do
                if ! kill -0 "$SERVICE_PID" 2>/dev/null; then
                    break
                fi
                sleep 1
                waited=$((waited + 1))
                echo -e "${YELLOW}   等待服务停止... ($waited/${max_wait})${NC}"
            done
            
            # 如果还在运行，强制杀掉
            if kill -0 "$SERVICE_PID" 2>/dev/null; then
                echo -e "${YELLOW}   强制停止服务...${NC}"
                kill -KILL "$SERVICE_PID" 2>/dev/null || true
                sleep 1
            fi
            
            echo -e "${GREEN}   服务已停止${NC}"
        fi
        
        # 清理标记文件
        rm -f "$update_stop_file" "$WORK_DIR/storage/update_marker" 2>/dev/null || true
        
        # 返回特殊值通知主循环需要重启
        return 2
    fi
    
    return 0
}

# 检查并停止已运行的守护进程
check_and_stop_existing_daemon() {
    local daemon_pid_file="$WORK_DIR/storage/centag.daemon.pid"
    local found_pids=""
    
    # 检查PID文件
    if [ -f "$daemon_pid_file" ]; then
        local old_daemon_pid=$(cat "$daemon_pid_file" 2>/dev/null || true)
        if [ -n "$old_daemon_pid" ] && kill -0 "$old_daemon_pid" 2>/dev/null; then
            found_pids="$old_daemon_pid"
        else
            rm -f "$daemon_pid_file" 2>/dev/null || true
        fi
    fi
    
    # 通过进程名查找其他守护进程实例
    if command -v pgrep >/dev/null 2>&1; then
        local script_name=$(basename "${BASH_SOURCE[0]}")
        local other_daemon_pids=$(pgrep -f "bash.*$script_name" 2>/dev/null | grep -v "^$$$" || true)
        if [ -n "$other_daemon_pids" ]; then
            for pid in $other_daemon_pids; do
                if kill -0 "$pid" 2>/dev/null; then
                    local cmdline=""
                    if [ -f "/proc/$pid/cmdline" ]; then
                        cmdline=$(cat "/proc/$pid/cmdline" 2>/dev/null | tr '\0' ' ' || echo "")
                    elif command -v ps >/dev/null 2>&1; then
                        cmdline=$(ps -p "$pid" -o cmd= 2>/dev/null || echo "")
                    fi
                    if echo "$cmdline" | grep -q "$script_name"; then
                        found_pids="$found_pids $pid"
                    fi
                fi
            done
        fi
    fi
    
    # 停止找到的守护进程
    if [ -n "$found_pids" ]; then
        echo -e "${YELLOW}⚠️  检测到已有 $SERVICE_NAME 守护进程在运行${NC}"
        for pid in $found_pids; do
            if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
                echo -e "${YELLOW}   发现守护进程 PID: $pid${NC}"
                echo -e "${YELLOW}   正在停止旧的守护进程...${NC}"
                
                kill -TERM "$pid" 2>/dev/null || true
                sleep 2
                
                if kill -0 "$pid" 2>/dev/null; then
                    echo -e "${YELLOW}   强制停止旧的守护进程...${NC}"
                    kill -KILL "$pid" 2>/dev/null || true
                    sleep 1
                fi
                
                if kill -0 "$pid" 2>/dev/null; then
                    echo -e "${RED}❌ 无法停止旧的守护进程，请手动终止 PID: $pid${NC}"
                    return 1
                else
                    echo -e "${GREEN}✅ 旧的守护进程已停止 (PID: $pid)${NC}"
                fi
            fi
        done
        
        rm -f "$daemon_pid_file" 2>/dev/null || true
    fi
    
    # 创建守护进程PID文件
    mkdir -p "$(dirname "$daemon_pid_file")" 2>/dev/null || true
    echo $$ > "$daemon_pid_file"
    
    return 0
}

# 主循环
main_loop() {
    local service_bin="$1"
    local restart_count=0
    DAEMON_EXIT=0
    
    # 检查并停止已运行的守护进程
    if ! check_and_stop_existing_daemon; then
        echo -e "${RED}❌ 无法启动守护进程，已有守护进程在运行${NC}"
        exit 1
    fi
    
    # 设置退出时清理守护进程PID文件
    local daemon_pid_file="$WORK_DIR/storage/centag.daemon.pid"
    trap "rm -f '$daemon_pid_file' 2>/dev/null || true; cleanup" SIGTERM SIGINT EXIT
    
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}🔄 $SERVICE_NAME 守护进程已启动${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "${BLUE}检查间隔: ${CHECK_INTERVAL} 秒${NC}"
    echo -e "${BLUE}按 Ctrl+C 停止守护进程${NC}"
    echo ""
    
    # 初始启动
    if ! start_service "$service_bin"; then
        restart_count=$((restart_count + 1))
        echo -e "${YELLOW}⚠️  初始启动失败，将在 ${CHECK_INTERVAL} 秒后重试 (第 $restart_count 次)...${NC}"
        sleep "$CHECK_INTERVAL"
    fi
    
    # Debug 模式：前台运行（直接执行，不进入主循环）
    if [ "$DAEMON_DEBUG" = "true" ]; then
        if [ -n "$SERVICE_PID" ] && check_service; then
            echo -e "${GREEN}✅ $SERVICE_NAME 已在 Debug 模式下启动 (PID: $SERVICE_PID, 端口: $PORT)${NC}"
            echo -e "${YELLOW}服务输出将显示在终端，按 Ctrl+C 停止服务${NC}"
            echo ""
        else
            echo -e "${YELLOW}⚠️  服务未成功启动${NC}"
        fi
    fi
    
    # 主循环
    while [ "$DAEMON_EXIT" -eq 0 ]; do
        if [ "$DAEMON_EXIT" -ne 0 ]; then
            break
        fi
        
        # 检查热更新标记
        local update_status=0
        check_update_marker
        update_status=$?
        
        # 如果检测到热更新，需要重新查找二进制文件并重启
        if [ $update_status -eq 2 ]; then
            echo -e "${GREEN}🔄 热更新完成，重新查找二进制文件...${NC}"
            
            SERVICE_PID=""
            if [ -f "$PID_FILE" ]; then
                rm -f "$PID_FILE"
            fi
            
            # 重新查找二进制文件（可能是新版本）
            local new_service_bin=$(find_service_binary)
            if [ -n "$new_service_bin" ]; then
                service_bin="$new_service_bin"
                echo -e "${GREEN}✅ 找到新版本 $SERVICE_NAME: $service_bin${NC}"
            fi
            
            # 短暂等待后启动
            sleep 1
            
            if [ "$DAEMON_EXIT" -eq 0 ]; then
                if start_service "$service_bin"; then
                    echo -e "${GREEN}✅ $SERVICE_NAME 已使用新版本启动${NC}"
                    restart_count=0
                else
                    echo -e "${RED}❌ $SERVICE_NAME 启动失败，将在 ${CHECK_INTERVAL} 秒后重试${NC}"
                    sleep "$CHECK_INTERVAL"
                fi
            fi
            continue
        fi
        
        # 检查进程是否还在运行
        if [ -z "$SERVICE_PID" ] || ! check_service; then
            if [ "$DAEMON_EXIT" -ne 0 ]; then
                break
            fi
            
            if [ -n "$SERVICE_PID" ]; then
                restart_count=$((restart_count + 1))
            fi
            
            if [ -n "$SERVICE_PID" ]; then
                echo -e "${YELLOW}⚠️  $SERVICE_NAME 进程已退出，准备重启 (第 $restart_count 次)...${NC}"
            else
                echo -e "${YELLOW}⚠️  $SERVICE_NAME 启动失败，准备重试 (第 $restart_count 次)...${NC}"
            fi
            
            SERVICE_PID=""
            if [ -f "$PID_FILE" ]; then
                rm -f "$PID_FILE"
            fi
            
            echo -e "${YELLOW}等待 ${CHECK_INTERVAL} 秒后重试...${NC}"
            sleep "$CHECK_INTERVAL"
            
            if [ "$DAEMON_EXIT" -eq 0 ]; then
                # 重新查找二进制文件
                echo -e "${YELLOW}重新查找二进制文件...${NC}"
                local new_service_bin=$(find_service_binary)
                if [ -n "$new_service_bin" ]; then
                    service_bin="$new_service_bin"
                    echo -e "${GREEN}✅ 找到 $SERVICE_NAME: $service_bin${NC}"
                fi
                
                if start_service "$service_bin"; then
                    echo -e "${GREEN}✅ $SERVICE_NAME 已启动${NC}"
                    if [ "$DAEMON_DEBUG" = "true" ] && [ -n "$SERVICE_PID" ] && kill -0 "$SERVICE_PID" 2>/dev/null; then
                        wait "$SERVICE_PID" 2>/dev/null || true
                        echo -e "${YELLOW}服务进程已退出${NC}"
                    fi
                else
                    echo -e "${RED}❌ $SERVICE_NAME 启动失败，将在 ${CHECK_INTERVAL} 秒后重试${NC}"
                    sleep "$CHECK_INTERVAL"
                fi
            fi
        else
            # 服务正在运行
            if [ "$DAEMON_DEBUG" = "true" ]; then
                wait "$SERVICE_PID" 2>/dev/null || true
                echo -e "${YELLOW}服务进程已退出${NC}"
            else
                sleep "$CHECK_INTERVAL"
            fi
        fi
    done
    
    # 退出前清理
    echo -e "${YELLOW}守护进程正在退出...${NC}"
    if [ -n "$SERVICE_PID" ] && kill -0 "$SERVICE_PID" 2>/dev/null; then
        echo -e "${YELLOW}正在停止 $SERVICE_NAME (PID: $SERVICE_PID)...${NC}"
        kill -TERM "$SERVICE_PID" 2>/dev/null || true
        sleep 1
        if kill -0 "$SERVICE_PID" 2>/dev/null; then
            kill -KILL "$SERVICE_PID" 2>/dev/null || true
        fi
    fi
    
    if [ -f "$PID_FILE" ]; then
        rm -f "$PID_FILE"
    fi
    
    daemon_pid_file="$WORK_DIR/storage/centag.daemon.pid"
    if [ -f "$daemon_pid_file" ]; then
        rm -f "$daemon_pid_file"
    fi
    
    echo -e "${GREEN}✅ 守护进程已退出${NC}"
}

# 主函数
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}🔄 $SERVICE_NAME 守护进程${NC}"
    echo -e "${BLUE}========================================${NC}"
    
    # 查找服务二进制文件
    local service_bin=$(find_service_binary)
    
    if [ -z "$service_bin" ]; then
        echo -e "${RED}❌ 错误：找不到 $SERVICE_NAME 二进制文件${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✅ 找到 $SERVICE_NAME: $service_bin${NC}"
    echo ""
    
    main_loop "$service_bin"
}

# 执行主函数
main "$@"
